package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

func TestQuotaNetNodeOverviewAggregatesSessions(t *testing.T) {
	now := time.Now().UTC()
	sessions := []registry.Session{
		{
			SessionID:          "sess-1",
			NodeID:             1,
			NodeKey:            "node-1",
			InstanceID:         "inst-1",
			WalletAddress:      "wallet-1",
			Status:             protocol.NodeStatusReady,
			CurrentConcurrency: 1,
			MaxConcurrency:     3,
			QueueSize:          2,
			MaxQueueSize:       5,
			LastHeartbeatAt:    now,
			Capabilities: []protocol.Capability{
				{Provider: "openai", Models: []string{"gpt-4.1", "gpt-4o"}},
			},
		},
		{
			SessionID:          "sess-2",
			NodeID:             2,
			NodeKey:            "node-2",
			InstanceID:         "inst-2",
			WalletAddress:      "wallet-2",
			Status:             protocol.NodeStatusBusy,
			CurrentConcurrency: 2,
			MaxConcurrency:     2,
			LastHeartbeatAt:    now.Add(-2 * time.Minute),
			Capabilities: []protocol.Capability{
				{Provider: "openai", Models: []string{"gpt-4.1"}},
				{Provider: "gemini", Models: []string{"gemini-2.5-pro"}},
			},
		},
	}

	got := quotaNetNodeOverview(sessions, map[string]int64{
		protocol.TaskStatusRunning: 2,
		protocol.TaskStatusSuccess: 9,
	})

	if got.Sessions.Total != 2 || got.Sessions.Connected != 2 || got.Sessions.Ready != 1 || got.Sessions.Busy != 1 {
		t.Fatalf("sessions = %+v", got.Sessions)
	}
	if got.Sessions.Stale != 1 {
		t.Fatalf("stale = %d, want 1", got.Sessions.Stale)
	}
	if got.Capacity.CurrentConcurrency != 3 || got.Capacity.MaxConcurrency != 5 || got.Capacity.Available != 2 || got.Capacity.QueueSize != 2 {
		t.Fatalf("capacity = %+v", got.Capacity)
	}
	if got.TaskStatuses[protocol.TaskStatusRunning] != 2 || got.TaskStatuses[protocol.TaskStatusSuccess] != 9 {
		t.Fatalf("task statuses = %+v", got.TaskStatuses)
	}
	if got.TaskStatuses[protocol.TaskStatusQueued] != 0 {
		t.Fatalf("queued status missing default: %+v", got.TaskStatuses)
	}
	if len(got.Providers) != 2 || got.Providers[0].Provider != "gemini" || got.Providers[1].Provider != "openai" {
		t.Fatalf("providers = %+v", got.Providers)
	}
	if len(got.Providers[1].Models) != 2 || got.Providers[1].Models[0] != "gpt-4.1" {
		t.Fatalf("openai models = %+v", got.Providers[1].Models)
	}
}
