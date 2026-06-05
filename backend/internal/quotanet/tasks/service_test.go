package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

func TestServiceDispatchAndWait(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	waiter := NewResponseWaiter()
	if err := reg.AttachSender("sess-1", &notifyingSender{waiter: waiter}); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	dispatcher := NewDispatcher(&stubStore{}, reg)
	svc := NewService(dispatcher, waiter)
	svc.newTaskID = func() string { return "task-1" }

	result, err := svc.DispatchAndWait(context.Background(), validInput())
	if err != nil {
		t.Fatalf("DispatchAndWait() error = %v", err)
	}
	if result.Task.TaskID != "task-1" || result.Response.TaskID != "task-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestServiceDispatchAndWaitDispatchFailure(t *testing.T) {
	svc := NewService(NewDispatcher(&stubStore{}, registry.New()), NewResponseWaiter())
	svc.newTaskID = func() string { return "task-1" }

	_, err := svc.DispatchAndWait(context.Background(), validInput())
	if !errors.Is(err, ErrNoNodeAvailable) {
		t.Fatalf("DispatchAndWait() error = %v, want ErrNoNodeAvailable", err)
	}
}

type notifyingSender struct {
	waiter *ResponseWaiter
}

func (s *notifyingSender) Send(_ context.Context, envelope protocol.Envelope) error {
	var dispatch protocol.TaskDispatch
	if err := envelope.DecodeData(&dispatch); err != nil {
		return err
	}
	go s.waiter.Notify(protocol.TaskResponse{
		TaskID: dispatch.TaskID,
		Status: protocol.TaskStatusSuccess,
		Usage:  protocol.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	})
	return nil
}

func TestServiceDispatchAndWaitTimeout(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(validSession("sess-1", 10)); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := reg.AttachSender("sess-1", &stubSender{}); err != nil {
		t.Fatalf("AttachSender() error = %v", err)
	}
	dispatcher := NewDispatcher(&stubStore{}, reg)
	svc := NewService(dispatcher, NewResponseWaiter())
	svc.newTaskID = func() string { return "task-1" }
	input := validInput()
	input.TimeoutSeconds = 1

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := svc.DispatchAndWait(ctx, input)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DispatchAndWait() error = %v, want context deadline", err)
	}
}
