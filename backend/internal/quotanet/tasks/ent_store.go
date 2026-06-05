package tasks

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/quotanettask"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
)

var ErrTaskNotFound = errors.New("quotanet task not found")

type EntStore struct {
	client *ent.Client
}

func NewEntStore(client *ent.Client) *EntStore {
	return &EntStore{client: client}
}

func (s *EntStore) CreateQueued(ctx context.Context, input CreateTaskInput, taskID string) (*Task, error) {
	if s == nil || s.client == nil {
		return nil, ErrTaskNotFound
	}
	row, err := s.client.QuotaNetTask.Create().
		SetTaskID(strings.TrimSpace(taskID)).
		SetRequestID(strings.TrimSpace(input.RequestID)).
		SetNillableUserID(input.UserID).
		SetNillableAPIKeyID(input.APIKeyID).
		SetNillableGroupID(input.GroupID).
		SetNillableAccountID(input.AccountID).
		SetPlatform(strings.TrimSpace(input.Platform)).
		SetEndpoint(strings.TrimSpace(input.Endpoint)).
		SetModel(strings.TrimSpace(input.Model)).
		SetStream(input.Stream).
		SetStatus(protocol.TaskStatusQueued).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return taskFromEnt(row), nil
}

func (s *EntStore) MarkDispatched(ctx context.Context, taskID string, candidate registry.Candidate, at time.Time) error {
	if s == nil || s.client == nil {
		return ErrTaskNotFound
	}
	affected, err := s.client.QuotaNetTask.Update().
		Where(quotanettask.TaskIDEQ(strings.TrimSpace(taskID))).
		SetNodeID(candidate.NodeID).
		SetSessionID(strings.TrimSpace(candidate.SessionID)).
		SetStatus(protocol.TaskStatusRunning).
		SetDispatchedAt(at).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (s *EntStore) AppendEvent(ctx context.Context, taskID, eventType string, sequence int64, payload map[string]any) error {
	if s == nil || s.client == nil {
		return ErrTaskNotFound
	}
	if payload == nil {
		payload = map[string]any{}
	}
	_, err := s.client.QuotaNetTaskEvent.Create().
		SetTaskID(strings.TrimSpace(taskID)).
		SetEventType(strings.TrimSpace(eventType)).
		SetSequence(sequence).
		SetPayload(payload).
		Save(ctx)
	return err
}

func (s *EntStore) MarkFailed(ctx context.Context, taskID, code, message string, at time.Time) error {
	if s == nil || s.client == nil {
		return ErrTaskNotFound
	}
	affected, err := s.client.QuotaNetTask.Update().
		Where(quotanettask.TaskIDEQ(strings.TrimSpace(taskID))).
		SetStatus(protocol.TaskStatusFailed).
		SetErrorCode(strings.TrimSpace(code)).
		SetErrorMessage(strings.TrimSpace(message)).
		SetCompletedAt(at).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func taskFromEnt(row *ent.QuotaNetTask) *Task {
	if row == nil {
		return nil
	}
	return &Task{
		ID:        row.ID,
		TaskID:    row.TaskID,
		RequestID: row.RequestID,
		NodeID:    row.NodeID,
		SessionID: row.SessionID,
		Platform:  row.Platform,
		Endpoint:  row.Endpoint,
		Model:     row.Model,
		Stream:    row.Stream,
		Status:    row.Status,
	}
}
