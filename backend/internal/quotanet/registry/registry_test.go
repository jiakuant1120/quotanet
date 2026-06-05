package registry

import (
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
	})
	if err != nil {
		t.Fatalf("UpdateHeartbeat() error = %v", err)
	}
	session, _ := reg.Get("sess-1")
	if session.WalletAddress != "wallet-new" || session.CurrentConcurrency != 2 || session.QueueSize != 1 {
		t.Fatalf("heartbeat not applied: %+v", session)
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
