package tasks

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
)

type DispatchResult struct {
	Task     *Task
	Response protocol.TaskResponse
}

type Service struct {
	dispatcher *Dispatcher
	waiter     *ResponseWaiter
	newTaskID  func() string
}

func NewService(dispatcher *Dispatcher, waiter *ResponseWaiter) *Service {
	return &Service{
		dispatcher: dispatcher,
		waiter:     waiter,
		newTaskID:  defaultTaskID,
	}
}

func (s *Service) DispatchAndWait(ctx context.Context, input CreateTaskInput) (*DispatchResult, error) {
	if s == nil || s.dispatcher == nil || s.waiter == nil {
		return nil, ErrInvalidTaskInput
	}
	timeout := time.Duration(input.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	taskID := s.newTaskID()
	subscription, err := s.waiter.Subscribe(taskID)
	if err != nil {
		return nil, err
	}
	defer subscription.Close()

	task, err := s.dispatcher.DispatchWithTaskID(ctx, input, taskID)
	if err != nil {
		return nil, err
	}
	response, err := subscription.Await(waitCtx)
	if err != nil {
		if waitCtx.Err() != nil {
			_ = s.dispatcher.MarkTimedOut(context.Background(), taskID)
		}
		return nil, err
	}
	return &DispatchResult{Task: task, Response: response}, nil
}
