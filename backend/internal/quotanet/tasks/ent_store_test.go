package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/quotanettask"
	"github.com/Wei-Shaw/sub2api/ent/quotanettaskevent"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"

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

func newTaskEntClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}
