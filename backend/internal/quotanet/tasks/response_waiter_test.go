package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

func TestResponseWaiterAwaitNotifiedResponse(t *testing.T) {
	waiter := NewResponseWaiter()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan protocol.TaskResponse, 1)
	errs := make(chan error, 1)
	go func() {
		response, err := waiter.Await(ctx, "task-1")
		if err != nil {
			errs <- err
			return
		}
		done <- response
	}()
	waitForWaiter(t, waiter, "task-1")

	waiter.Notify(protocol.TaskResponse{TaskID: "task-1", Status: protocol.TaskStatusSuccess})

	select {
	case err := <-errs:
		t.Fatalf("Await() error = %v", err)
	case response := <-done:
		if response.TaskID != "task-1" || response.Status != protocol.TaskStatusSuccess {
			t.Fatalf("response = %+v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestResponseWaiterAwaitContextCancelled(t *testing.T) {
	waiter := NewResponseWaiter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := waiter.Await(ctx, "task-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Await() error = %v, want context.Canceled", err)
	}
}

func TestResponseRecorderStoresThenNotifies(t *testing.T) {
	store := &stubResponseStore{}
	waiter := NewResponseWaiter()
	recorder := NewResponseRecorder(store, waiter)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan protocol.TaskResponse, 1)
	go func() {
		response, err := waiter.Await(ctx, "task-1")
		if err == nil {
			done <- response
		}
	}()
	waitForWaiter(t, waiter, "task-1")

	response := protocol.TaskResponse{TaskID: "task-1", Status: protocol.TaskStatusSuccess}
	if err := recorder.TaskResponseReceived(context.Background(), "sess-1", response, time.Unix(100, 0)); err != nil {
		t.Fatalf("TaskResponseReceived() error = %v", err)
	}
	if store.sessionID != "sess-1" || store.response.TaskID != "task-1" {
		t.Fatalf("store session=%q response=%+v", store.sessionID, store.response)
	}

	select {
	case got := <-done:
		if got.TaskID != "task-1" {
			t.Fatalf("notified response = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestResponseRecorderDoesNotNotifyOnStoreError(t *testing.T) {
	store := &stubResponseStore{err: errors.New("store failed")}
	waiter := NewResponseWaiter()
	recorder := NewResponseRecorder(store, waiter)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := waiter.Await(ctx, "task-1")
		done <- err
	}()
	waitForWaiter(t, waiter, "task-1")

	err := recorder.TaskResponseReceived(context.Background(), "sess-1", protocol.TaskResponse{TaskID: "task-1", Status: protocol.TaskStatusSuccess}, time.Now())
	if err == nil {
		t.Fatal("TaskResponseReceived() error = nil, want store error")
	}
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Await() error = %v, want context deadline", err)
	}
}

type stubResponseStore struct {
	sessionID string
	response  protocol.TaskResponse
	at        time.Time
	err       error
}

func (s *stubResponseStore) TaskResponseReceived(_ context.Context, sessionID string, response protocol.TaskResponse, at time.Time) error {
	if s.err != nil {
		return s.err
	}
	s.sessionID = sessionID
	s.response = response
	s.at = at
	return nil
}

func waitForWaiter(t *testing.T, waiter *ResponseWaiter, taskID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		waiter.mu.Lock()
		count := len(waiter.waiters[taskID])
		waiter.mu.Unlock()
		if count > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("waiter for %q was not registered", taskID)
}
