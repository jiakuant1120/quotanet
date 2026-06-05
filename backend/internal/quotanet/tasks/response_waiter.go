package tasks

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

var ErrResponseWaiterClosed = errors.New("quotanet response waiter is closed")

type TaskResponseStore interface {
	TaskResponseReceived(ctx context.Context, sessionID string, response protocol.TaskResponse, at time.Time) error
}

type ResponseWaiter struct {
	mu      sync.Mutex
	closed  bool
	waiters map[string][]chan protocol.TaskResponse
}

func NewResponseWaiter() *ResponseWaiter {
	return &ResponseWaiter{waiters: make(map[string][]chan protocol.TaskResponse)}
}

func (w *ResponseWaiter) Await(ctx context.Context, taskID string) (protocol.TaskResponse, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return protocol.TaskResponse{}, ErrInvalidTaskInput
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ch := make(chan protocol.TaskResponse, 1)
	if err := w.add(taskID, ch); err != nil {
		return protocol.TaskResponse{}, err
	}
	defer w.remove(taskID, ch)

	select {
	case <-ctx.Done():
		return protocol.TaskResponse{}, ctx.Err()
	case response := <-ch:
		return response, nil
	}
}

func (w *ResponseWaiter) Notify(response protocol.TaskResponse) {
	if w == nil {
		return
	}
	taskID := strings.TrimSpace(response.TaskID)
	if taskID == "" {
		return
	}

	w.mu.Lock()
	waiters := append([]chan protocol.TaskResponse(nil), w.waiters[taskID]...)
	delete(w.waiters, taskID)
	w.mu.Unlock()

	for _, ch := range waiters {
		select {
		case ch <- response:
		default:
		}
	}
}

func (w *ResponseWaiter) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	waiters := w.waiters
	w.waiters = make(map[string][]chan protocol.TaskResponse)
	w.mu.Unlock()

	for _, chans := range waiters {
		for _, ch := range chans {
			close(ch)
		}
	}
}

func (w *ResponseWaiter) add(taskID string, ch chan protocol.TaskResponse) error {
	if w == nil {
		return ErrResponseWaiterClosed
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrResponseWaiterClosed
	}
	w.waiters[taskID] = append(w.waiters[taskID], ch)
	return nil
}

func (w *ResponseWaiter) remove(taskID string, ch chan protocol.TaskResponse) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	waiters := w.waiters[taskID]
	for i, waiter := range waiters {
		if waiter == ch {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(waiters) == 0 {
		delete(w.waiters, taskID)
		return
	}
	w.waiters[taskID] = waiters
}

type ResponseRecorder struct {
	store  TaskResponseStore
	waiter *ResponseWaiter
}

func NewResponseRecorder(store TaskResponseStore, waiter *ResponseWaiter) *ResponseRecorder {
	return &ResponseRecorder{store: store, waiter: waiter}
}

func (r *ResponseRecorder) TaskResponseReceived(ctx context.Context, sessionID string, response protocol.TaskResponse, at time.Time) error {
	if r == nil {
		return ErrInvalidTaskInput
	}
	if r.store != nil {
		if err := r.store.TaskResponseReceived(ctx, sessionID, response, at); err != nil {
			return err
		}
	}
	if r.waiter != nil {
		r.waiter.Notify(response)
	}
	return nil
}
