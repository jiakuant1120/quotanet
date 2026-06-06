package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := New()
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	reg.now = func() time.Time { return now }

	err := reg.Register(Session{
		SessionID:     " sess-1 ",
		NodeID:        12,
		NodeKey:       "node-a",
		InstanceID:    "inst-a",
		WalletAddress: "wallet-a",
		Capabilities: []protocol.Capability{
			{Provider: "openai", Models: []string{"gpt-4.1"}, MaxConcurrency: 2},
		},
		MaxConcurrency: 2,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	session, ok := reg.Get("sess-1")
	if !ok {
		t.Fatal("Get() did not find registered session")
	}
	if session.Status != protocol.NodeStatusReady {
		t.Fatalf("status = %q, want ready", session.Status)
	}
	if !session.ConnectedAt.Equal(now) || !session.LastHeartbeatAt.Equal(now) {
		t.Fatalf("timestamps were not initialized from clock")
	}
}

func TestRegistryHeartbeatAndUnregister(t *testing.T) {
	reg := New()
	if err := reg.Register(validSession("sess-1", 1, 3)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := reg.UpdateHeartbeat("sess-1", protocol.ClientHeartbeat{
		WalletAddress:      "wallet-new",
		Status:             protocol.NodeStatusBusy,
		CurrentConcurrency: 2,
		MaxConcurrency:     3,
		QueueSize:          1,
		MaxQueueSize:       10,
		Capabilities: []protocol.Capability{
			{Provider: " openai ", Models: []string{" gpt-5 ", ""}, MaxConcurrency: 3},
			{Provider: "", Models: []string{"ignored"}},
		},
		Accounts: []protocol.AccountHeartbeat{
			{Provider: " openai ", Status: " ready ", CurrentConcurrency: 1, MaxConcurrency: 3, Models: []string{" gpt-4.1 ", ""}},
			{Provider: "", Status: "ready", Models: []string{"ignored"}},
		},
	})
	if err != nil {
		t.Fatalf("UpdateHeartbeat() error = %v", err)
	}
	session, _ := reg.Get("sess-1")
	if session.WalletAddress != "wallet-new" || session.CurrentConcurrency != 2 || session.QueueSize != 1 {
		t.Fatalf("heartbeat not applied: %+v", session)
	}
	if len(session.Capabilities) != 1 || session.Capabilities[0].Provider != "openai" || session.Capabilities[0].Models[0] != "gpt-5" {
		t.Fatalf("capabilities not normalized: %+v", session.Capabilities)
	}
	if candidates := reg.Candidates("openai", "gpt-5", time.Minute); len(candidates) != 1 {
		t.Fatalf("Candidates(gpt-5) = %+v, want updated heartbeat capability", candidates)
	}
	if len(session.Accounts) != 1 || session.Accounts[0].Provider != "openai" || session.Accounts[0].Status != "ready" || session.Accounts[0].Models[0] != "gpt-4.1" {
		t.Fatalf("account heartbeats not normalized: %+v", session.Accounts)
	}

	if err := reg.Unregister("sess-1", "closed"); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	session, _ = reg.Get("sess-1")
	if session.Status != protocol.NodeStatusOffline || session.DisconnectedAt == nil || session.CloseReason != "closed" {
		t.Fatalf("session not marked offline: %+v", session)
	}
}

func TestRegistryCandidatesFilterAndSort(t *testing.T) {
	reg := New()
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	reg.now = func() time.Time { return now }

	fast := validSession("fast", 1, 5)
	fast.CurrentConcurrency = 1
	fast.QueueSize = 2
	fast.LastHeartbeatAt = now.Add(-1 * time.Second)
	slow := validSession("slow", 2, 3)
	slow.CurrentConcurrency = 1
	slow.QueueSize = 0
	slow.LastHeartbeatAt = now
	full := validSession("full", 3, 1)
	full.CurrentConcurrency = 1
	stale := validSession("stale", 4, 3)
	stale.LastHeartbeatAt = now.Add(-2 * time.Minute)
	offline := validSession("offline", 5, 3)
	disconnectedAt := now
	offline.DisconnectedAt = &disconnectedAt

	for _, session := range []Session{fast, slow, full, stale, offline} {
		if err := reg.Register(session); err != nil {
			t.Fatalf("Register(%s) error = %v", session.SessionID, err)
		}
	}

	candidates := reg.Candidates("openai", "gpt-4.1", 30*time.Second)
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2: %+v", len(candidates), candidates)
	}
	if candidates[0].SessionID != "fast" {
		t.Fatalf("first candidate = %q, want fast: %+v", candidates[0].SessionID, candidates)
	}
	if candidates[1].SessionID != "slow" {
		t.Fatalf("second candidate = %q, want slow: %+v", candidates[1].SessionID, candidates)
	}
}

func TestRegistryCandidatesRequireModelMatch(t *testing.T) {
	reg := New()
	if err := reg.Register(validSession("sess-1", 1, 3)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := reg.Candidates("openai", "unknown-model", time.Minute); len(got) != 0 {
		t.Fatalf("Candidates(unknown model) = %+v, want none", got)
	}
}

func TestRegistryAvailableModels(t *testing.T) {
	reg := New()
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	reg.now = func() time.Time { return now }

	first := validSession("first", 1, 3)
	first.Capabilities = []protocol.Capability{
		{Provider: "openai", Models: []string{"gpt-4.1", "gpt-4o-mini"}},
		{Provider: "gemini", Models: []string{"gemini-2.5-pro"}},
	}
	second := validSession("second", 2, 2)
	second.Capabilities = []protocol.Capability{
		{Provider: "openai", Models: []string{"GPT-4.1", "gpt-5"}},
	}
	full := validSession("full", 3, 1)
	full.CurrentConcurrency = 1
	full.Capabilities = []protocol.Capability{{Provider: "openai", Models: []string{"hidden-model"}}}
	stale := validSession("stale", 4, 3)
	stale.LastHeartbeatAt = now.Add(-2 * time.Minute)
	stale.Capabilities = []protocol.Capability{{Provider: "openai", Models: []string{"stale-model"}}}

	for _, session := range []Session{first, second, full, stale} {
		if err := reg.Register(session); err != nil {
			t.Fatalf("Register(%s) error = %v", session.SessionID, err)
		}
	}

	models := reg.AvailableModels("openai", 30*time.Second)
	want := []string{"gpt-4.1", "gpt-4o-mini", "gpt-5"}
	if len(models) != len(want) {
		t.Fatalf("models = %+v, want %+v", models, want)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Fatalf("models = %+v, want %+v", models, want)
		}
	}
}

func TestRegistryInvalidInputs(t *testing.T) {
	reg := New()
	if err := reg.Register(Session{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Register(empty) error = %v, want ErrInvalidSession", err)
	}
	if err := reg.UpdateHeartbeat("missing", protocol.ClientHeartbeat{}); err == nil {
		t.Fatal("UpdateHeartbeat(invalid heartbeat) error = nil")
	}
	if err := reg.Unregister("", ""); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Unregister(empty) error = %v, want ErrInvalidSession", err)
	}
}

func TestRegistrySenderLifecycle(t *testing.T) {
	reg := New()
	if err := reg.Register(validSession("sess-1", 1, 3)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	sender := &stubSender{}
	if err := reg.AttachSender("sess-1", sender); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	envelope, err := protocol.NewEnvelope(protocol.EventTaskDispatch, "msg-1", protocol.TaskDispatch{
		TaskID:   "task-1",
		Provider: "openai",
		Model:    "gpt-4.1",
		Payload:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := reg.Send(context.Background(), "sess-1", envelope); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if sender.sent.MsgID != "msg-1" {
		t.Fatalf("sent envelope = %+v", sender.sent)
	}

	if err := reg.Unregister("sess-1", "closed"); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if err := reg.Send(context.Background(), "sess-1", envelope); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Send(after unregister) error = %v, want ErrInvalidSession", err)
	}
}

func TestRegistrySendRequiresSender(t *testing.T) {
	reg := New()
	if err := reg.Register(validSession("sess-1", 1, 3)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	envelope, err := protocol.NewEnvelope(protocol.EventTaskCancel, "msg-1", protocol.TaskCancel{TaskID: "task-1"})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := reg.Send(context.Background(), "sess-1", envelope); !errors.Is(err, ErrSenderNotFound) {
		t.Fatalf("Send(no sender) error = %v, want ErrSenderNotFound", err)
	}
}

func validSession(sessionID string, nodeID int64, maxConcurrency int) Session {
	return Session{
		SessionID:          sessionID,
		NodeID:             nodeID,
		NodeKey:            "node-" + sessionID,
		InstanceID:         "inst-" + sessionID,
		WalletAddress:      "wallet-" + sessionID,
		Status:             protocol.NodeStatusReady,
		MaxConcurrency:     maxConcurrency,
		CurrentConcurrency: 0,
		Capabilities: []protocol.Capability{
			{Provider: "openai", Models: []string{"gpt-4.1", "gpt-4o-mini"}, MaxConcurrency: maxConcurrency},
		},
		LastHeartbeatAt: time.Now(),
	}
}

type stubSender struct {
	sent protocol.Envelope
}

func (s *stubSender) Send(_ context.Context, envelope protocol.Envelope) error {
	s.sent = envelope
	return nil
}
