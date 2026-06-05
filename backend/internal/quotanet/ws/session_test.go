package ws

import (
	"context"
	"errors"
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

	ack, session, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", envelope)
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

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", envelope)
	if !errors.Is(err, protocol.ErrUnsupportedVersion) {
		t.Fatalf("HandleHello() error = %v, want ErrUnsupportedVersion", err)
	}
	assertAck(t, ack, AckStatusError)
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

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", envelope)
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

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", envelope)
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

	ack, _, err := manager.HandleHello(context.Background(), "sess-1", "inst-1", "token", envelope)
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

func helloEnvelope(t *testing.T, hello protocol.ClientHello) protocol.Envelope {
	t.Helper()
	envelope, err := protocol.NewEnvelope(protocol.EventClientHello, "msg-1", hello)
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
