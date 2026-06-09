package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/quotanetcontributionledger"
	"github.com/Wei-Shaw/sub2api/ent/quotanettask"
	"github.com/Wei-Shaw/sub2api/ent/quotanettaskevent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestEntStoreTaskLifecycle(t *testing.T) {
	client := newTaskEntClient(t)
	store := NewEntStore(client)
	ctx := context.Background()

	task, err := store.CreateQueued(ctx, validInput(), "task-1")
	if err != nil {
		t.Fatalf("CreateQueued() error = %v", err)
	}
	if task.TaskID != "task-1" || task.Status != protocol.TaskStatusQueued {
		t.Fatalf("created task = %+v", task)
	}

	dispatchedAt := time.Unix(100, 0).UTC()
	candidate := registry.Candidate{NodeID: 7, SessionID: "sess-1"}
	if err := store.MarkDispatched(ctx, "task-1", candidate, dispatchedAt); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}
	if err := store.AppendEvent(ctx, "task-1", protocol.EventTaskDispatch, 1, map[string]any{"node_id": 7}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	row, err := client.QuotaNetTask.Query().Where(quotanettask.TaskIDEQ("task-1")).Only(ctx)
	if err != nil {
		t.Fatalf("query task error = %v", err)
	}
	if row.Status != protocol.TaskStatusRunning || row.NodeID == nil || *row.NodeID != 7 || row.SessionID == nil || *row.SessionID != "sess-1" {
		t.Fatalf("dispatched row = %+v", row)
	}
	if row.DispatchedAt == nil || !row.DispatchedAt.Equal(dispatchedAt) {
		t.Fatalf("dispatched_at = %v, want %v", row.DispatchedAt, dispatchedAt)
	}

	events, err := client.QuotaNetTaskEvent.Query().Where(quotanettaskevent.TaskIDEQ("task-1")).All(ctx)
	if err != nil {
		t.Fatalf("query events error = %v", err)
	}
	if len(events) != 1 || events[0].EventType != protocol.EventTaskDispatch {
		t.Fatalf("events = %+v", events)
	}
}

func TestEntStoreMarkFailed(t *testing.T) {
	client := newTaskEntClient(t)
	store := NewEntStore(client)
	ctx := context.Background()

	if _, err := store.CreateQueued(ctx, validInput(), "task-1"); err != nil {
		t.Fatalf("CreateQueued() error = %v", err)
	}
	completedAt := time.Unix(200, 0).UTC()
	if err := store.MarkFailed(ctx, "task-1", "NO_NODE_AVAILABLE", "no node", completedAt); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	row, err := client.QuotaNetTask.Query().Where(quotanettask.TaskIDEQ("task-1")).Only(ctx)
	if err != nil {
		t.Fatalf("query task error = %v", err)
	}
	if row.Status != protocol.TaskStatusFailed || row.ErrorCode == nil || *row.ErrorCode != "NO_NODE_AVAILABLE" {
		t.Fatalf("failed row = %+v", row)
	}
	if row.CompletedAt == nil || !row.CompletedAt.Equal(completedAt) {
		t.Fatalf("completed_at = %v, want %v", row.CompletedAt, completedAt)
	}
}

func TestEntStoreTaskResponseReceivedIsIdempotent(t *testing.T) {
	client := newTaskEntClient(t)
	store := NewEntStore(client)
	ctx := context.Background()

	if _, err := store.CreateQueued(ctx, validInput(), "task-1"); err != nil {
		t.Fatalf("CreateQueued() error = %v", err)
	}
	node, err := client.QuotaNetNode.Create().
		SetNodeKey("node-key-1").
		SetWalletAddress("wallet-1").
		SetTokenHash("token-hash").
		Save(ctx)
	if err != nil {
		t.Fatalf("create node error = %v", err)
	}
	candidate := registry.Candidate{NodeID: node.ID, SessionID: "sess-1"}
	if err := store.MarkDispatched(ctx, "task-1", candidate, time.Unix(100, 0).UTC()); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}

	response := protocol.TaskResponse{
		TaskID: "task-1",
		Status: protocol.TaskStatusSuccess,
		Usage:  protocol.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}
	if err := store.TaskResponseReceived(ctx, "sess-1", response, time.Unix(200, 0).UTC()); err != nil {
		t.Fatalf("TaskResponseReceived() first error = %v", err)
	}
	if err := store.TaskResponseReceived(ctx, "sess-1", response, time.Unix(201, 0).UTC()); !errors.Is(err, ErrDuplicateTaskResponse) {
		t.Fatalf("TaskResponseReceived() duplicate error = %v, want ErrDuplicateTaskResponse", err)
	}

	events, err := client.QuotaNetTaskEvent.Query().Where(quotanettaskevent.TaskIDEQ("task-1")).All(ctx)
	if err != nil {
		t.Fatalf("query events error = %v", err)
	}
	if len(events) != 1 || events[0].EventType != protocol.EventTaskResponse {
		t.Fatalf("events = %+v, want one task_response event", events)
	}

	ledgers, err := client.QuotaNetContributionLedger.Query().
		Where(quotanetcontributionledger.TaskIDEQ("task-1")).
		All(ctx)
	if err != nil {
		t.Fatalf("query contribution ledger error = %v", err)
	}
	if len(ledgers) != 1 {
		t.Fatalf("contribution ledgers = %+v, want one ledger", ledgers)
	}
	if ledgers[0].NodeID != node.ID || ledgers[0].WalletAddress != "wallet-1" {
		t.Fatalf("ledger node fields = %+v", ledgers[0])
	}
	if ledgers[0].TokenFlow != 3 || ledgers[0].Status != protocol.SettlementStatusPending {
		t.Fatalf("ledger settlement fields = %+v", ledgers[0])
	}
}

func TestEntStoreTaskResponseReceivedSkipsLedgerForFailure(t *testing.T) {
	client := newTaskEntClient(t)
	store := NewEntStore(client)
	ctx := context.Background()

	if _, err := store.CreateQueued(ctx, validInput(), "task-1"); err != nil {
		t.Fatalf("CreateQueued() error = %v", err)
	}
	node, err := client.QuotaNetNode.Create().
		SetNodeKey("node-key-1").
		SetWalletAddress("wallet-1").
		SetTokenHash("token-hash").
		Save(ctx)
	if err != nil {
		t.Fatalf("create node error = %v", err)
	}
	candidate := registry.Candidate{NodeID: node.ID, SessionID: "sess-1"}
	if err := store.MarkDispatched(ctx, "task-1", candidate, time.Unix(100, 0).UTC()); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}

	response := protocol.TaskResponse{
		TaskID:       "task-1",
		Status:       protocol.TaskStatusFailed,
		ErrorCode:    "UPSTREAM_ERROR",
		ErrorMessage: "upstream failed",
	}
	if err := store.TaskResponseReceived(ctx, "sess-1", response, time.Unix(200, 0).UTC()); err != nil {
		t.Fatalf("TaskResponseReceived() error = %v", err)
	}

	count, err := client.QuotaNetContributionLedger.Query().
		Where(quotanetcontributionledger.TaskIDEQ("task-1")).
		Count(ctx)
	if err != nil {
		t.Fatalf("count contribution ledger error = %v", err)
	}
	if count != 0 {
		t.Fatalf("contribution ledger count = %d, want 0", count)
	}
}

func TestEntStoreContributionUsesStandardCostNotBuyerMultiplier(t *testing.T) {
	client := newTaskEntClient(t)
	billingService := service.NewBillingService(&config.Config{}, nil)
	store := NewEntStore(client).WithContributionBilling(billingService, nil)
	ctx := context.Background()

	group, err := client.Group.Create().
		SetName("quotanet-standard-cost").
		SetPlatform("openai").
		SetRateMultiplier(2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create group error = %v", err)
	}
	input := validInput()
	input.UserID = ptrInt64(101)
	input.GroupID = &group.ID
	input.Model = "gpt-5.4"
	if _, err := store.CreateQueued(ctx, input, "task-1"); err != nil {
		t.Fatalf("CreateQueued() error = %v", err)
	}
	node, err := client.QuotaNetNode.Create().
		SetNodeKey("node-key-1").
		SetWalletAddress("wallet-1").
		SetTokenHash("token-hash").
		Save(ctx)
	if err != nil {
		t.Fatalf("create node error = %v", err)
	}
	if err := store.MarkDispatched(ctx, "task-1", registry.Candidate{NodeID: node.ID, SessionID: "sess-1"}, time.Unix(100, 0).UTC()); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}

	response := protocol.TaskResponse{
		TaskID: "task-1",
		Status: protocol.TaskStatusSuccess,
		Usage:  protocol.Usage{PromptTokens: 1000, CompletionTokens: 100, TotalTokens: 1100},
	}
	if err := store.TaskResponseReceived(ctx, "sess-1", response, time.Unix(200, 0).UTC()); err != nil {
		t.Fatalf("TaskResponseReceived() error = %v", err)
	}
	ledger, err := client.QuotaNetContributionLedger.Query().
		Where(quotanetcontributionledger.TaskIDEQ("task-1")).
		Only(ctx)
	if err != nil {
		t.Fatalf("query contribution ledger error = %v", err)
	}

	const wantStandard = 1000*2.5e-6 + 100*1.5e-5
	if diff := ledger.StandardCostUsd - wantStandard; diff < -1e-12 || diff > 1e-12 {
		t.Fatalf("standard_cost_usd = %.12f, want %.12f", ledger.StandardCostUsd, wantStandard)
	}
	if diff := ledger.ContributionUsd - ledger.StandardCostUsd; diff < -1e-12 || diff > 1e-12 {
		t.Fatalf("contribution_usd = %.12f, want standard_cost_usd %.12f", ledger.ContributionUsd, ledger.StandardCostUsd)
	}
	if diff := ledger.ActualCostUsd - ledger.StandardCostUsd*2; diff < -1e-12 || diff > 1e-12 {
		t.Fatalf("actual_cost_usd = %.12f, want %.12f", ledger.ActualCostUsd, ledger.StandardCostUsd*2)
	}
}

func TestEntStoreMarkRunningTimedOutBefore(t *testing.T) {
	client := newTaskEntClient(t)
	store := NewEntStore(client)
	ctx := context.Background()

	if _, err := store.CreateQueued(ctx, validInput(), "old-task"); err != nil {
		t.Fatalf("CreateQueued(old) error = %v", err)
	}
	if _, err := store.CreateQueued(ctx, validInput(), "fresh-task"); err != nil {
		t.Fatalf("CreateQueued(fresh) error = %v", err)
	}
	candidate := registry.Candidate{NodeID: 1, SessionID: "sess-1"}
	if err := store.MarkDispatched(ctx, "old-task", candidate, time.Unix(100, 0).UTC()); err != nil {
		t.Fatalf("MarkDispatched(old) error = %v", err)
	}
	if err := store.MarkDispatched(ctx, "fresh-task", candidate, time.Unix(300, 0).UTC()); err != nil {
		t.Fatalf("MarkDispatched(fresh) error = %v", err)
	}

	result, err := store.MarkRunningTimedOutBefore(ctx, time.Unix(200, 0).UTC(), time.Unix(400, 0).UTC(), 100)
	if err != nil {
		t.Fatalf("MarkRunningTimedOutBefore() error = %v", err)
	}
	if result.Count != 1 || len(result.TaskIDs) != 1 || result.TaskIDs[0] != "old-task" {
		t.Fatalf("result = %+v, want old-task only", result)
	}
	oldRow, err := client.QuotaNetTask.Query().Where(quotanettask.TaskIDEQ("old-task")).Only(ctx)
	if err != nil {
		t.Fatalf("query old task error = %v", err)
	}
	if oldRow.Status != protocol.TaskStatusTimeout || oldRow.ErrorCode == nil || *oldRow.ErrorCode != "TIMEOUT_SWEEP" {
		t.Fatalf("old task = %+v, want timeout", oldRow)
	}
	freshRow, err := client.QuotaNetTask.Query().Where(quotanettask.TaskIDEQ("fresh-task")).Only(ctx)
	if err != nil {
		t.Fatalf("query fresh task error = %v", err)
	}
	if freshRow.Status != protocol.TaskStatusRunning {
		t.Fatalf("fresh task status = %q, want running", freshRow.Status)
	}
	events, err := client.QuotaNetTaskEvent.Query().Where(quotanettaskevent.TaskIDEQ("old-task")).All(ctx)
	if err != nil {
		t.Fatalf("query events error = %v", err)
	}
	if len(events) != 1 || events[0].EventType != protocol.EventTaskTimeout {
		t.Fatalf("events = %+v, want one task_timeout event", events)
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

func newTaskEntClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name()))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_group_rate_multipliers (
		user_id INTEGER NOT NULL,
		group_id INTEGER NOT NULL,
		rate_multiplier REAL,
		rpm_override INTEGER
	)`); err != nil {
		t.Fatalf("create user_group_rate_multipliers: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
