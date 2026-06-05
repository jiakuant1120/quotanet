package ws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

func TestSessionManagerHandleHelloRegistersSession(t *testing.T) {
	reg := registry.New()
	auth := &stubAuthenticator{
		node: AuthenticatedNode{
			NodeID:        42,
			NodeKey:       "node-42",
			WalletAddress: "wallet-42",
		},
	}
	manager := NewSessionManager(auth, reg)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	envelope := helloEnvelope(t, protocol.ClientHello{
		ClientID:        "client-1",
		ClientVersion:   "0.1.0",
		WalletAddress:   "wallet-42",
		ProtocolVersion: protocol.Version,
		Capabilities: []protocol.Capability{
			{Provider: " openai ", Models: []string{" gpt-4.1 ", ""}, MaxConcurrency: 3},
			{Provider: "", Models: []string{"ignored"}, MaxConcurrency: 99},
		},
	})

	ack, session, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", "127.0.0.1", envelope)
	if err != nil {
		t.Fatalf("HandleHello() error = %v", err)
	}
	assertAck(t, ack, AckStatusOK)
	if session.NodeID != 42 || session.NodeKey != "node-42" || session.MaxConcurrency != 3 {
		t.Fatalf("session = %+v", session)
	}

	stored, ok := reg.Get("sess-1")
	if !ok {
		t.Fatal("session was not registered")
	}
	if stored.ClientVersion != "0.1.0" || len(stored.Capabilities) != 1 {
		t.Fatalf("stored session = %+v", stored)
	}
	if stored.Capabilities[0].Provider != "openai" || stored.Capabilities[0].Models[0] != "gpt-4.1" {
		t.Fatalf("capabilities not normalized: %+v", stored.Capabilities)
	}
}

func TestSessionManagerHandleHelloRejectsUnsupportedVersion(t *testing.T) {
	manager := NewSessionManager(&stubAuthenticator{}, registry.New())
	envelope := helloEnvelope(t, protocol.ClientHello{
		ClientID:        "client-1",
		WalletAddress:   "wallet-1",
		ProtocolVersion: "old",
	})

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", "", envelope)
	if !errors.Is(err, protocol.ErrUnsupportedVersion) {
		t.Fatalf("HandleHello() error = %v, want ErrUnsupportedVersion", err)
	}
	assertAck(t, ack, AckStatusError)
	message := ackMessage(t, ack)
	if !strings.Contains(message, "client=old") || !strings.Contains(message, "server="+protocol.Version) {
		t.Fatalf("ack message = %q, want explicit client/server protocol versions", message)
	}
}

func TestSessionManagerHandleHelloRejectsUnexpectedEvent(t *testing.T) {
	manager := NewSessionManager(&stubAuthenticator{}, registry.New())
	envelope, err := protocol.NewEnvelope(protocol.EventClientHeartbeat, "msg-1", protocol.ClientHeartbeat{
		WalletAddress:      "wallet-1",
		Status:             protocol.NodeStatusReady,
		CurrentConcurrency: 0,
		MaxConcurrency:     1,
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", "", envelope)
	if !errors.Is(err, ErrUnexpectedEvent) {
		t.Fatalf("HandleHello() error = %v, want ErrUnexpectedEvent", err)
	}
	assertAck(t, ack, AckStatusError)
}

func TestSessionManagerHandleHelloRejectsAuthenticatorError(t *testing.T) {
	manager := NewSessionManager(&stubAuthenticator{err: errors.New("bad token")}, registry.New())
	envelope := helloEnvelope(t, protocol.ClientHello{
		ClientID:        "client-1",
		WalletAddress:   "wallet-1",
		ProtocolVersion: protocol.Version,
	})

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", "", envelope)
	if !errors.Is(err, ErrNodeRejected) {
		t.Fatalf("HandleHello() error = %v, want ErrNodeRejected", err)
	}
	assertAck(t, ack, AckStatusError)
}

func TestSessionManagerHandleHelloRejectsWalletMismatch(t *testing.T) {
	manager := NewSessionManager(&stubAuthenticator{
		node: AuthenticatedNode{NodeID: 1, NodeKey: "node-1", WalletAddress: "registered-wallet"},
	}, registry.New())
	envelope := helloEnvelope(t, protocol.ClientHello{
		ClientID:        "client-1",
		WalletAddress:   "hello-wallet",
		ProtocolVersion: protocol.Version,
	})

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", "", envelope)
	if !errors.Is(err, ErrNodeRejected) {
		t.Fatalf("HandleHello() error = %v, want ErrNodeRejected", err)
	}
	assertAck(t, ack, AckStatusError)
}

func TestSessionManagerHandleHeartbeatUpdatesRegistry(t *testing.T) {
	reg := registry.New()
	manager := NewSessionManager(&stubAuthenticator{}, reg)
	if err := reg.Register(registry.Session{
		SessionID:      "sess-1",
		NodeID:         1,
		NodeKey:        "node-1",
		InstanceID:     "inst-1",
		WalletAddress:  "wallet-1",
		MaxConcurrency: 1,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	envelope, err := protocol.NewEnvelope(protocol.EventClientHeartbeat, "msg-2", protocol.ClientHeartbeat{
		WalletAddress:      "wallet-1",
		Status:             protocol.NodeStatusBusy,
		CurrentConcurrency: 1,
		MaxConcurrency:     2,
		QueueSize:          3,
		MaxQueueSize:       5,
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	ack, err := manager.HandleHeartbeat("sess-1", envelope)
	if err != nil {
		t.Fatalf("HandleHeartbeat() error = %v", err)
	}
	assertAck(t, ack, AckStatusOK)
	session, _ := reg.Get("sess-1")
	if session.Status != protocol.NodeStatusBusy || session.MaxConcurrency != 2 || session.QueueSize != 3 {
		t.Fatalf("heartbeat not applied: %+v", session)
	}
}

func TestSessionManagerHandleTaskResponsePersistsResult(t *testing.T) {
	store := &stubTaskStore{}
	manager := NewSessionManager(&stubAuthenticator{}, registry.New()).WithTaskStore(store)
	now := time.Date(2026, 6, 5, 13, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	envelope, err := protocol.NewEnvelope(protocol.EventTaskResponse, "msg-task", protocol.TaskResponse{
		TaskID: "task-1",
		Status: protocol.TaskStatusSuccess,
		Usage:  protocol.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	ack, err := manager.HandleTaskResponse(context.Background(), "sess-1", envelope)
	if err != nil {
		t.Fatalf("HandleTaskResponse() error = %v", err)
	}
	assertAck(t, ack, AckStatusOK)
	if store.sessionID != "sess-1" || store.response.TaskID != "task-1" || !store.at.Equal(now) {
		t.Fatalf("task store session=%q response=%+v at=%v", store.sessionID, store.response, store.at)
	}
}

func TestSessionManagerHandleTaskResponseRequiresStore(t *testing.T) {
	manager := NewSessionManager(&stubAuthenticator{}, registry.New())
	envelope, err := protocol.NewEnvelope(protocol.EventTaskResponse, "msg-task", protocol.TaskResponse{
		TaskID: "task-1",
		Status: protocol.TaskStatusSuccess,
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	ack, err := manager.HandleTaskResponse(context.Background(), "sess-1", envelope)
	if !errors.Is(err, ErrUnexpectedEvent) {
		t.Fatalf("HandleTaskResponse() error = %v, want ErrUnexpectedEvent", err)
	}
	assertAck(t, ack, AckStatusError)
}

func TestSessionManagerPersistsLifecycle(t *testing.T) {
	reg := registry.New()
	store := &stubSessionStore{}
	manager := NewSessionManager(&stubAuthenticator{
		node: AuthenticatedNode{
			NodeID:        42,
			NodeKey:       "node-42",
			WalletAddress: "wallet-42",
		},
	}, reg).WithSessionStore(store)
	envelope := helloEnvelope(t, protocol.ClientHello{
		ClientID:        "node-42",
		WalletAddress:   "wallet-42",
		ProtocolVersion: protocol.Version,
	})

	_, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", "10.0.0.1", envelope)
	if err != nil {
		t.Fatalf("HandleHello() error = %v", err)
	}
	if store.connected.SessionID != "sess-1" || store.remoteAddr != "10.0.0.1" {
		t.Fatalf("connected session=%+v remote=%q", store.connected, store.remoteAddr)
	}

	heartbeat := heartbeatEnvelope(t)
	if _, err := manager.HandleHeartbeat("sess-1", heartbeat); err != nil {
		t.Fatalf("HandleHeartbeat() error = %v", err)
	}
	if store.heartbeatSessionID != "sess-1" || store.heartbeat.Status != protocol.NodeStatusBusy {
		t.Fatalf("heartbeat session=%q payload=%+v", store.heartbeatSessionID, store.heartbeat)
	}

	manager.Disconnect(context.Background(), "sess-1", "closed")
	if store.disconnectedSessionID != "sess-1" || store.disconnectReason != "closed" {
		t.Fatalf("disconnect session=%q reason=%q", store.disconnectedSessionID, store.disconnectReason)
	}
}

func helloEnvelope(t *testing.T, hello protocol.ClientHello) protocol.Envelope {
	t.Helper()
	envelope, err := protocol.NewEnvelope(protocol.EventClientHello, "msg-1", hello)
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	return envelope
}

func heartbeatEnvelope(t *testing.T) protocol.Envelope {
	t.Helper()
	envelope, err := protocol.NewEnvelope(protocol.EventClientHeartbeat, "msg-heartbeat", protocol.ClientHeartbeat{
		WalletAddress:      "wallet-42",
		Status:             protocol.NodeStatusBusy,
		CurrentConcurrency: 1,
		MaxConcurrency:     2,
		QueueSize:          3,
		MaxQueueSize:       5,
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	return envelope
}

func assertAck(t *testing.T, envelope protocol.Envelope, status string) {
	t.Helper()
	if envelope.Event != protocol.EventServerAck {
		t.Fatalf("ack event = %q, want %q", envelope.Event, protocol.EventServerAck)
	}
	var ack protocol.Ack
	if err := envelope.DecodeData(&ack); err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	if ack.Status != status {
		t.Fatalf("ack status = %q, want %q, message=%q", ack.Status, status, ack.Message)
	}
}

func ackMessage(t *testing.T, envelope protocol.Envelope) string {
	t.Helper()
	var ack protocol.Ack
	if err := envelope.DecodeData(&ack); err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	return ack.Message
}

type stubAuthenticator struct {
	node AuthenticatedNode
	err  error
}

func (s *stubAuthenticator) AuthenticateNode(_ context.Context, _ string, _ protocol.ClientHello) (AuthenticatedNode, error) {
	if s.err != nil {
		return AuthenticatedNode{}, s.err
	}
	if s.node.NodeID == 0 {
		return AuthenticatedNode{NodeID: 1, NodeKey: "node-1", WalletAddress: "wallet-1"}, nil
	}
	return s.node, nil
}

type stubSessionStore struct {
	connected             registry.Session
	remoteAddr            string
	heartbeatSessionID    string
	heartbeat             protocol.ClientHeartbeat
	disconnectedSessionID string
	disconnectReason      string
}

func (s *stubSessionStore) SessionConnected(_ context.Context, session registry.Session, remoteAddr string) error {
	s.connected = session
	s.remoteAddr = remoteAddr
	return nil
}

func (s *stubSessionStore) SessionHeartbeat(_ context.Context, sessionID string, heartbeat protocol.ClientHeartbeat, _ time.Time) error {
	s.heartbeatSessionID = sessionID
	s.heartbeat = heartbeat
	return nil
}

func (s *stubSessionStore) SessionDisconnected(_ context.Context, sessionID, reason string, _ time.Time) error {
	s.disconnectedSessionID = sessionID
	s.disconnectReason = reason
	return nil
}

type stubTaskStore struct {
	sessionID string
	response  protocol.TaskResponse
	at        time.Time
	err       error
}

func (s *stubTaskStore) TaskResponseReceived(_ context.Context, sessionID string, response protocol.TaskResponse, at time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.sessionID = sessionID
	s.response = response
	s.at = at
	return nil
}
