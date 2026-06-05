package tasks

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/quotanettask"
	"github.com/Wei-Shaw/sub2api/ent/quotanettaskevent"
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

func (s *EntStore) TaskResponseReceived(ctx context.Context, sessionID string, response protocol.TaskResponse, at time.Time) error {
	if s == nil || s.client == nil {
		return ErrTaskNotFound
	}
	if err := response.Validate(); err != nil {
		return err
	}
	taskID := strings.TrimSpace(response.TaskID)
	sessionID = strings.TrimSpace(sessionID)

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	update := tx.QuotaNetTask.Update().
		Where(
			quotanettask.TaskIDEQ(taskID),
			quotanettask.SessionIDEQ(sessionID),
			quotanettask.StatusEQ(protocol.TaskStatusRunning),
			quotanettask.CompletedAtIsNil(),
		).
		SetStatus(strings.TrimSpace(response.Status)).
		SetPromptTokens(response.Usage.PromptTokens).
		SetCompletionTokens(response.Usage.CompletionTokens).
		SetTotalTokens(response.Usage.TotalTokens).
		SetFirstTokenMs(response.FirstTokenMS).
		SetDurationMs(response.DurationMS).
		SetCompletedAt(at)
	if strings.TrimSpace(response.ErrorCode) != "" {
		update.SetErrorCode(strings.TrimSpace(response.ErrorCode))
	} else {
		update.ClearErrorCode()
	}
	if strings.TrimSpace(response.ErrorMessage) != "" {
		update.SetErrorMessage(strings.TrimSpace(response.ErrorMessage))
	} else {
		update.ClearErrorMessage()
	}

	affected, err := update.Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTaskNotFound
	}

	count, err := tx.QuotaNetTaskEvent.Query().
		Where(quotanettaskevent.TaskIDEQ(taskID)).
		Count(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.QuotaNetTaskEvent.Create().
		SetTaskID(taskID).
		SetEventType(protocol.EventTaskResponse).
		SetSequence(int64(count + 1)).
		SetPayload(taskResponseEventPayload(sessionID, response)).
		SetCreatedAt(at).
		Save(ctx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func taskResponseEventPayload(sessionID string, response protocol.TaskResponse) map[string]any {
	return map[string]any{
		"session_id": sessionID,
		"status":     strings.TrimSpace(response.Status),
		"error_code": strings.TrimSpace(response.ErrorCode),
		"error_msg":  strings.TrimSpace(response.ErrorMessage),
		"usage": map[string]any{
			"prompt_tokens":     response.Usage.PromptTokens,
			"completion_tokens": response.Usage.CompletionTokens,
			"total_tokens":      response.Usage.TotalTokens,
		},
		"duration_ms":    response.DurationMS,
		"first_token_ms": response.FirstTokenMS,
		"payload":        response.Payload,
	}
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
