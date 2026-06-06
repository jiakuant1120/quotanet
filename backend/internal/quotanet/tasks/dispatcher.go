// Package tasks dispatches QuotaNet task requests to connected nodes.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

var (
	ErrInvalidTaskInput = errors.New("invalid quotanet task input")
	ErrNoNodeAvailable  = errors.New("no quotanet node available")
)

type CreateTaskInput struct {
	RequestID      string
	UserID         *int64
	APIKeyID       *int64
	GroupID        *int64
	AccountID      *int64
	NodeID         *int64
	Platform       string
	Endpoint       string
	Model          string
	Stream         bool
	TimeoutSeconds int
	Payload        map[string]any
}

type Task struct {
	ID               int64
	TaskID           string
	RequestID        string
	UserID           *int64
	APIKeyID         *int64
	GroupID          *int64
	AccountID        *int64
	NodeID           *int64
	SessionID        *string
	Platform         string
	Endpoint         string
	Model            string
	Stream           bool
	Status           string
	ErrorCode        *string
	ErrorMessage     *string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	FirstTokenMS     *int
	DurationMS       *int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DispatchedAt     *time.Time
	CompletedAt      *time.Time
}

type TaskEvent struct {
	ID        int64
	TaskID    string
	EventType string
	Sequence  int64
	Payload   map[string]any
	CreatedAt time.Time
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type TimeoutSweepResult struct {
	Count   int64    `json:"count"`
	TaskIDs []string `json:"task_ids"`
}

type ListParams struct {
	Page      int
	PageSize  int
	Status    string
	Platform  string
	NodeID    *int64
	AccountID *int64
	UserID    *int64
	APIKeyID  *int64
	GroupID   *int64
	Search    string
}

type Store interface {
	CreateQueued(ctx context.Context, input CreateTaskInput, taskID string) (*Task, error)
	MarkDispatched(ctx context.Context, taskID string, candidate registry.Candidate, at time.Time) error
	AppendEvent(ctx context.Context, taskID, eventType string, sequence int64, payload map[string]any) error
	MarkFailed(ctx context.Context, taskID, code, message string, at time.Time) error
}

type Dispatcher struct {
	store      Store
	registry   *registry.Registry
	now        func() time.Time
	newTaskID  func() string
	newMessage func() string
	staleAfter time.Duration
}

func NewDispatcher(store Store, reg *registry.Registry) *Dispatcher {
	return &Dispatcher{
		store:      store,
		registry:   reg,
		now:        time.Now,
		newTaskID:  defaultTaskID,
		newMessage: defaultMessageID,
		staleAfter: 60 * time.Second,
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, input CreateTaskInput) (*Task, error) {
	if input.NodeID != nil {
		return d.DispatchToNodeID(ctx, input, *input.NodeID)
	}
	return d.DispatchWithTaskID(ctx, input, d.newTaskID())
}

func (d *Dispatcher) DispatchToNodeID(ctx context.Context, input CreateTaskInput, nodeID int64) (*Task, error) {
	if nodeID <= 0 {
		return nil, fmt.Errorf("%w: node_id must be positive", ErrInvalidTaskInput)
	}
	return d.dispatch(ctx, input, d.newTaskID(), nodeID)
}

func (d *Dispatcher) DispatchWithTaskID(ctx context.Context, input CreateTaskInput, taskID string) (*Task, error) {
	nodeID := int64(0)
	if input.NodeID != nil {
		nodeID = *input.NodeID
	}
	return d.dispatch(ctx, input, taskID, nodeID)
}

func (d *Dispatcher) dispatch(ctx context.Context, input CreateTaskInput, taskID string, nodeID int64) (*Task, error) {
	if err := validateCreateInput(input); err != nil {
		return nil, err
	}
	if d == nil || d.store == nil || d.registry == nil {
		return nil, ErrInvalidTaskInput
	}
	if nodeID <= 0 && input.NodeID != nil {
		return nil, fmt.Errorf("%w: node_id must be positive", ErrInvalidTaskInput)
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("%w: task_id is required", ErrInvalidTaskInput)
	}

	task, err := d.store.CreateQueued(ctx, input, taskID)
	if err != nil {
		return nil, err
	}

	candidates := d.registry.Candidates(input.Platform, input.Model, d.staleAfter)
	if len(candidates) == 0 {
		_ = d.store.MarkFailed(ctx, taskID, "NO_NODE_AVAILABLE", ErrNoNodeAvailable.Error(), d.now())
		return nil, ErrNoNodeAvailable
	}
	candidate, ok := chooseCandidate(candidates, nodeID)
	if !ok {
		_ = d.store.MarkFailed(ctx, taskID, "NO_NODE_AVAILABLE", ErrNoNodeAvailable.Error(), d.now())
		return nil, ErrNoNodeAvailable
	}
	if err := d.store.MarkDispatched(ctx, taskID, candidate, d.now()); err != nil {
		return nil, err
	}

	dispatch := protocol.TaskDispatch{
		TaskID:         taskID,
		Provider:       input.Platform,
		Model:          input.Model,
		Endpoint:       input.Endpoint,
		Stream:         input.Stream,
		TimeoutSeconds: input.TimeoutSeconds,
		Payload:        input.Payload,
	}
	envelope, err := protocol.NewEnvelope(protocol.EventTaskDispatch, d.newMessage(), dispatch)
	if err != nil {
		return nil, err
	}
	if err := d.registry.Send(ctx, candidate.SessionID, envelope); err != nil {
		_ = d.store.MarkFailed(ctx, taskID, "DISPATCH_SEND_FAILED", err.Error(), d.now())
		return nil, err
	}
	if err := d.store.AppendEvent(ctx, taskID, protocol.EventTaskDispatch, 1, map[string]any{
		"session_id": candidate.SessionID,
		"node_id":    candidate.NodeID,
		"provider":   input.Platform,
		"model":      input.Model,
	}); err != nil {
		return nil, err
	}

	task.NodeID = &candidate.NodeID
	task.SessionID = &candidate.SessionID
	task.Status = protocol.TaskStatusRunning
	return task, nil
}

func chooseCandidate(candidates []registry.Candidate, nodeID int64) (registry.Candidate, bool) {
	if len(candidates) == 0 {
		return registry.Candidate{}, false
	}
	if nodeID <= 0 {
		return candidates[0], true
	}
	for _, candidate := range candidates {
		if candidate.NodeID == nodeID {
			return candidate, true
		}
	}
	return registry.Candidate{}, false
}

func (d *Dispatcher) MarkTimedOut(ctx context.Context, taskID string) error {
	if d == nil || d.store == nil {
		return ErrInvalidTaskInput
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("%w: task_id is required", ErrInvalidTaskInput)
	}
	return d.store.MarkFailed(ctx, taskID, "TIMEOUT", "quotanet task timed out", d.now())
}

func validateCreateInput(input CreateTaskInput) error {
	if strings.TrimSpace(input.RequestID) == "" {
		return fmt.Errorf("%w: request_id is required", ErrInvalidTaskInput)
	}
	if strings.TrimSpace(input.Platform) == "" {
		return fmt.Errorf("%w: platform is required", ErrInvalidTaskInput)
	}
	if strings.TrimSpace(input.Endpoint) == "" {
		return fmt.Errorf("%w: endpoint is required", ErrInvalidTaskInput)
	}
	if strings.TrimSpace(input.Model) == "" {
		return fmt.Errorf("%w: model is required", ErrInvalidTaskInput)
	}
	if input.TimeoutSeconds < 0 {
		return fmt.Errorf("%w: timeout_seconds must be non-negative", ErrInvalidTaskInput)
	}
	if input.NodeID != nil && *input.NodeID <= 0 {
		return fmt.Errorf("%w: node_id must be positive", ErrInvalidTaskInput)
	}
	return nil
}

func defaultTaskID() string {
	return "qnt_" + randomID()
}

func defaultMessageID() string {
	return "qnm_" + randomID()
}
