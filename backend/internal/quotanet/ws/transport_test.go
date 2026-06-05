package ws

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/auth"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

func TestSessionManagerServeHelloHeartbeatAndClose(t *testing.T) {
	token, hash := tokenAndHash(t)
	reg := registry.New()
	manager := NewSessionManager(&transportAuthenticator{hash: hash}, reg)
	conn := &scriptedConn{
		read: []protocol.Envelope{
			helloForTransport(t),
			heartbeatForTransport(t, protocol.NodeStatusBusy),
		},
		readErr: io.EOF,
	}

	err := manager.Serve(context.Background(), conn, ServeOptions{
		SessionID:  "sess-1",
		InstanceID: "inst-1",
		Token:      token,
	})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Serve() error = %v, want io.EOF", err)
	}
	if !conn.closed {
		t.Fatal("connection was not closed")
	}
	if len(conn.written) != 2 {
		t.Fatalf("written ack count = %d, want 2", len(conn.written))
	}
	assertAck(t, conn.written[0], AckStatusOK)
	assertAck(t, conn.written[1], AckStatusOK)

	session, ok := reg.Get("sess-1")
	if !ok {
		t.Fatal("session missing after close")
	}
	if session.Status != protocol.NodeStatusOffline || session.DisconnectedAt == nil {
		t.Fatalf("session not marked offline: %+v", session)
	}
}

func TestSessionManagerServeAttachesSender(t *testing.T) {
	token, hash := tokenAndHash(t)
	reg := registry.New()
	manager := NewSessionManager(&transportAuthenticator{hash: hash}, reg)
	conn := NewSerialConn(&scriptedConn{
		read:    []protocol.Envelope{helloForTransport(t)},
		readErr: io.EOF,
	})

	err := manager.Serve(context.Background(), conn, ServeOptions{
		SessionID:  "sess-1",
		InstanceID: "inst-1",
		Token:      token,
	})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Serve() error = %v, want io.EOF", err)
	}
	envelope, err := protocol.NewEnvelope(protocol.EventTaskCancel, "msg-cancel", protocol.TaskCancel{TaskID: "task-1"})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := reg.Send(context.Background(), "sess-1", envelope); !errors.Is(err, registry.ErrInvalidSession) {
		t.Fatalf("Send(after close) error = %v, want ErrInvalidSession", err)
	}
}

func TestSessionManagerServeStopsAfterRejectedHello(t *testing.T) {
	manager := NewSessionManager(&transportAuthenticator{err: errors.New("nope")}, registry.New())
	conn := &scriptedConn{read: []protocol.Envelope{helloForTransport(t)}}

	err := manager.Serve(context.Background(), conn, ServeOptions{
		SessionID:  "sess-1",
		InstanceID: "inst-1",
		Token:      "bad-token",
	})
	if !errors.Is(err, ErrNodeRejected) {
		t.Fatalf("Serve() error = %v, want ErrNodeRejected", err)
	}
	if len(conn.written) != 1 {
		t.Fatalf("written ack count = %d, want 1", len(conn.written))
	}
	assertAck(t, conn.written[0], AckStatusError)
	if _, ok := manager.Registry().Get("sess-1"); ok {
		t.Fatal("rejected hello should not register session")
	}
}

func TestSessionManagerServeAcceptsTaskResponse(t *testing.T) {
	token, hash := tokenAndHash(t)
	taskStore := &stubTaskStore{}
	manager := NewSessionManager(&transportAuthenticator{hash: hash}, registry.New()).WithTaskStore(taskStore)
	response, err := protocol.NewEnvelope(protocol.EventTaskResponse, "msg-x", protocol.TaskResponse{
		TaskID: "task-1",
		Status: protocol.TaskStatusSuccess,
		Usage:  protocol.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	conn := &scriptedConn{read: []protocol.Envelope{helloForTransport(t), response}, readErr: io.EOF}

	err = manager.Serve(context.Background(), conn, ServeOptions{
		SessionID:  "sess-1",
		InstanceID: "inst-1",
		Token:      token,
	})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Serve() error = %v, want io.EOF", err)
	}
	if len(conn.written) != 2 {
		t.Fatalf("written ack count = %d, want 2", len(conn.written))
	}
	assertAck(t, conn.written[1], AckStatusOK)
	if taskStore.sessionID != "sess-1" || taskStore.response.TaskID != "task-1" {
		t.Fatalf("task store session=%q response=%+v", taskStore.sessionID, taskStore.response)
	}
}

func TestSessionManagerServeRejectsUnsupportedEvent(t *testing.T) {
	token, hash := tokenAndHash(t)
	manager := NewSessionManager(&transportAuthenticator{hash: hash}, registry.New())
	unsupported, err := protocol.NewEnvelope(protocol.EventTaskDelta, "msg-x", map[string]any{"task_id": "task-1"})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	conn := &scriptedConn{read: []protocol.Envelope{helloForTransport(t), unsupported}}

	err = manager.Serve(context.Background(), conn, ServeOptions{
		SessionID:  "sess-1",
		InstanceID: "inst-1",
		Token:      token,
	})
	if !errors.Is(err, ErrUnexpectedEvent) {
		t.Fatalf("Serve() error = %v, want ErrUnexpectedEvent", err)
	}
	if len(conn.written) != 2 {
		t.Fatalf("written ack count = %d, want 2", len(conn.written))
	}
	assertAck(t, conn.written[1], AckStatusError)
}

func TestSessionManagerServeNilConn(t *testing.T) {
	manager := NewSessionManager(&transportAuthenticator{}, registry.New())
	if err := manager.Serve(context.Background(), nil, ServeOptions{}); !errors.Is(err, ErrNilTransport) {
		t.Fatalf("Serve(nil) error = %v, want ErrNilTransport", err)
	}
}

func helloForTransport(t *testing.T) protocol.Envelope {
	t.Helper()
	envelope, err := protocol.NewEnvelope(protocol.EventClientHello, "msg-hello", protocol.ClientHello{
		ClientID:        "node-1",
		ClientVersion:   "0.1.0",
		WalletAddress:   "wallet-1",
		ProtocolVersion: protocol.Version,
		Capabilities: []protocol.Capability{
			{Provider: "openai", Models: []string{"gpt-4.1"}, MaxConcurrency: 2},
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	return envelope
}

func heartbeatForTransport(t *testing.T, status string) protocol.Envelope {
	t.Helper()
	envelope, err := protocol.NewEnvelope(protocol.EventClientHeartbeat, "msg-heartbeat", protocol.ClientHeartbeat{
		WalletAddress:      "wallet-1",
		Status:             status,
		CurrentConcurrency: 1,
		MaxConcurrency:     2,
		QueueSize:          0,
		MaxQueueSize:       5,
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	return envelope
}

func tokenAndHash(t *testing.T) (string, string) {
	t.Helper()
	token, err := auth.GenerateNodeToken()
	if err != nil {
		t.Fatalf("GenerateNodeToken() error = %v", err)
	}
	hash, err := auth.HashNodeToken(token)
	if err != nil {
		t.Fatalf("HashNodeToken() error = %v", err)
	}
	return token, hash
}

type scriptedConn struct {
	read    []protocol.Envelope
	readErr error
	written []protocol.Envelope
	closed  bool
}

func (c *scriptedConn) ReadJSON(v any) error {
	if len(c.read) == 0 {
		if c.readErr != nil {
			return c.readErr
		}
		return io.EOF
	}
	next := c.read[0]
	c.read = c.read[1:]
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (c *scriptedConn) WriteJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var envelope protocol.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	c.written = append(c.written, envelope)
	return nil
}

func (c *scriptedConn) Close() error {
	c.closed = true
	return nil
}

type transportAuthenticator struct {
	hash string
	err  error
}

func (a *transportAuthenticator) AuthenticateNode(_ context.Context, token string, _ protocol.ClientHello) (AuthenticatedNode, error) {
	if a.err != nil {
		return AuthenticatedNode{}, a.err
	}
	if err := auth.VerifyNodeToken(token, a.hash); err != nil {
		return AuthenticatedNode{}, err
	}
	return AuthenticatedNode{
		NodeID:        1,
		NodeKey:       "node-1",
		WalletAddress: "wallet-1",
	}, nil
}
