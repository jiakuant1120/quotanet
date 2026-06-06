package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/tasks"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type QuotaNetTaskHandler struct {
	store      *tasks.EntStore
	dispatcher *tasks.Dispatcher
	service    *tasks.Service
}

func NewQuotaNetTaskHandler(store *tasks.EntStore, dispatcher *tasks.Dispatcher, service *tasks.Service) *QuotaNetTaskHandler {
	return &QuotaNetTaskHandler{store: store, dispatcher: dispatcher, service: service}
}

type quotaNetTaskResponse struct {
	ID               int64   `json:"id"`
	TaskID           string  `json:"task_id"`
	RequestID        string  `json:"request_id"`
	UserID           *int64  `json:"user_id,omitempty"`
	APIKeyID         *int64  `json:"api_key_id,omitempty"`
	GroupID          *int64  `json:"group_id,omitempty"`
	AccountID        *int64  `json:"account_id,omitempty"`
	NodeID           *int64  `json:"node_id,omitempty"`
	SessionID        *string `json:"session_id,omitempty"`
	Platform         string  `json:"platform"`
	Endpoint         string  `json:"endpoint"`
	Model            string  `json:"model"`
	Stream           bool    `json:"stream"`
	Status           string  `json:"status"`
	ErrorCode        *string `json:"error_code,omitempty"`
	ErrorMessage     *string `json:"error_message,omitempty"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	FirstTokenMS     *int    `json:"first_token_ms,omitempty"`
	DurationMS       *int    `json:"duration_ms,omitempty"`
	CreatedAt        string  `json:"created_at,omitempty"`
	UpdatedAt        string  `json:"updated_at,omitempty"`
	DispatchedAt     *string `json:"dispatched_at,omitempty"`
	CompletedAt      *string `json:"completed_at,omitempty"`
}

type quotaNetTaskEventResponse struct {
	ID        int64          `json:"id"`
	TaskID    string         `json:"task_id"`
	EventType string         `json:"event_type"`
	Sequence  int64          `json:"sequence"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"created_at,omitempty"`
}

type quotaNetTaskDispatchRequest struct {
	RequestID      string         `json:"request_id"`
	UserID         *int64         `json:"user_id"`
	APIKeyID       *int64         `json:"api_key_id"`
	GroupID        *int64         `json:"group_id"`
	AccountID      *int64         `json:"account_id"`
	NodeID         *int64         `json:"node_id"`
	Platform       string         `json:"platform" binding:"required,max=50"`
	Endpoint       string         `json:"endpoint" binding:"required,max=100"`
	Model          string         `json:"model" binding:"required,max=100"`
	Stream         bool           `json:"stream"`
	TimeoutSeconds int            `json:"timeout_seconds" binding:"omitempty,min=0"`
	Payload        map[string]any `json:"payload"`
}

type quotaNetTaskDispatchSyncResponse struct {
	Task     *quotaNetTaskResponse     `json:"task"`
	Response tasksProtocolTaskResponse `json:"response"`
}

type tasksProtocolTaskResponse struct {
	TaskID       string         `json:"task_id"`
	Status       string         `json:"status"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_msg,omitempty"`
	Usage        map[string]int `json:"usage"`
	Payload      map[string]any `json:"payload,omitempty"`
	DurationMS   int            `json:"duration_ms,omitempty"`
	FirstTokenMS int            `json:"first_token_ms,omitempty"`
}

func (h *QuotaNetTaskHandler) List(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet task service is not initialized")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params, ok := quotaNetTaskListParams(c, page, pageSize)
	if !ok {
		return
	}
	items, total, err := h.store.List(c.Request.Context(), params)
	if err != nil {
		quotaNetTaskError(c, err)
		return
	}
	out := make([]*quotaNetTaskResponse, 0, len(items))
	for _, item := range items {
		out = append(out, quotaNetTaskToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *QuotaNetTaskHandler) Dispatch(c *gin.Context) {
	if h == nil || h.dispatcher == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet task dispatcher is not initialized")
		return
	}
	input, ok := quotaNetTaskDispatchInput(c)
	if !ok {
		return
	}
	task, err := h.dispatcher.Dispatch(c.Request.Context(), input)
	if err != nil {
		quotaNetTaskError(c, err)
		return
	}
	response.Created(c, quotaNetTaskToResponse(task))
}

func (h *QuotaNetTaskHandler) DispatchSync(c *gin.Context) {
	if h == nil || h.service == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet task service is not initialized")
		return
	}
	input, ok := quotaNetTaskDispatchInput(c)
	if !ok {
		return
	}
	result, err := h.service.DispatchAndWait(c.Request.Context(), input)
	if err != nil {
		quotaNetTaskError(c, err)
		return
	}
	response.Success(c, quotaNetTaskDispatchSyncResponse{
		Task:     quotaNetTaskToResponse(result.Task),
		Response: quotaNetTaskResponsePayload(result.Response),
	})
}

func (h *QuotaNetTaskHandler) Get(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet task service is not initialized")
		return
	}
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		response.BadRequest(c, "invalid task id")
		return
	}
	task, err := h.store.Get(c.Request.Context(), taskID)
	if err != nil {
		quotaNetTaskError(c, err)
		return
	}
	response.Success(c, quotaNetTaskToResponse(task))
}

func (h *QuotaNetTaskHandler) Events(c *gin.Context) {
	if h == nil || h.store == nil {
		response.Error(c, http.StatusServiceUnavailable, "quotanet task service is not initialized")
		return
	}
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		response.BadRequest(c, "invalid task id")
		return
	}
	events, err := h.store.Events(c.Request.Context(), taskID)
	if err != nil {
		quotaNetTaskError(c, err)
		return
	}
	out := make([]*quotaNetTaskEventResponse, 0, len(events))
	for _, event := range events {
		out = append(out, quotaNetTaskEventToResponse(event))
	}
	response.Success(c, gin.H{"items": out})
}

func quotaNetTaskListParams(c *gin.Context, page, pageSize int) (tasks.ListParams, bool) {
	params := tasks.ListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
		Platform: strings.TrimSpace(c.Query("platform")),
		Search:   strings.TrimSpace(c.Query("search")),
	}
	var ok bool
	if params.NodeID, ok = optionalInt64Query(c, "node_id"); !ok {
		return tasks.ListParams{}, false
	}
	if params.AccountID, ok = optionalInt64Query(c, "account_id"); !ok {
		return tasks.ListParams{}, false
	}
	if params.UserID, ok = optionalInt64Query(c, "user_id"); !ok {
		return tasks.ListParams{}, false
	}
	if params.APIKeyID, ok = optionalInt64Query(c, "api_key_id"); !ok {
		return tasks.ListParams{}, false
	}
	if params.GroupID, ok = optionalInt64Query(c, "group_id"); !ok {
		return tasks.ListParams{}, false
	}
	return params, true
}

func quotaNetTaskDispatchInput(c *gin.Context) (tasks.CreateTaskInput, bool) {
	var req quotaNetTaskDispatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return tasks.CreateTaskInput{}, false
	}
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID = "admin_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return tasks.CreateTaskInput{
		RequestID:      requestID,
		UserID:         req.UserID,
		APIKeyID:       req.APIKeyID,
		GroupID:        req.GroupID,
		AccountID:      req.AccountID,
		NodeID:         req.NodeID,
		Platform:       strings.TrimSpace(req.Platform),
		Endpoint:       strings.TrimSpace(req.Endpoint),
		Model:          strings.TrimSpace(req.Model),
		Stream:         req.Stream,
		TimeoutSeconds: req.TimeoutSeconds,
		Payload:        req.Payload,
	}, true
}

func optionalInt64Query(c *gin.Context, key string) (*int64, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+key)
		return nil, false
	}
	return &id, true
}

func quotaNetTaskResponsePayload(resp protocol.TaskResponse) tasksProtocolTaskResponse {
	return tasksProtocolTaskResponse{
		TaskID:       resp.TaskID,
		Status:       resp.Status,
		ErrorCode:    resp.ErrorCode,
		ErrorMessage: resp.ErrorMessage,
		Usage: map[string]int{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
		},
		Payload:      resp.Payload,
		DurationMS:   resp.DurationMS,
		FirstTokenMS: resp.FirstTokenMS,
	}
}

func quotaNetTaskError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tasks.ErrTaskNotFound):
		response.NotFound(c, "quotanet task not found")
	case errors.Is(err, tasks.ErrInvalidTaskInput):
		response.BadRequest(c, "invalid quotanet task input")
	case errors.Is(err, tasks.ErrNoNodeAvailable):
		response.Error(c, http.StatusServiceUnavailable, "no quotanet node available")
	default:
		response.Error(c, http.StatusInternalServerError, "quotanet task operation failed")
	}
}

func quotaNetTaskToResponse(task *tasks.Task) *quotaNetTaskResponse {
	if task == nil {
		return nil
	}
	resp := &quotaNetTaskResponse{
		ID:               task.ID,
		TaskID:           task.TaskID,
		RequestID:        task.RequestID,
		UserID:           task.UserID,
		APIKeyID:         task.APIKeyID,
		GroupID:          task.GroupID,
		AccountID:        task.AccountID,
		NodeID:           task.NodeID,
		SessionID:        task.SessionID,
		Platform:         task.Platform,
		Endpoint:         task.Endpoint,
		Model:            task.Model,
		Stream:           task.Stream,
		Status:           task.Status,
		ErrorCode:        task.ErrorCode,
		ErrorMessage:     task.ErrorMessage,
		PromptTokens:     task.PromptTokens,
		CompletionTokens: task.CompletionTokens,
		TotalTokens:      task.TotalTokens,
		FirstTokenMS:     task.FirstTokenMS,
		DurationMS:       task.DurationMS,
		CreatedAt:        formatQuotaNetTime(task.CreatedAt),
		UpdatedAt:        formatQuotaNetTime(task.UpdatedAt),
		DispatchedAt:     quotaNetOptionalTime(task.DispatchedAt),
		CompletedAt:      quotaNetOptionalTime(task.CompletedAt),
	}
	return resp
}

func quotaNetTaskEventToResponse(event *tasks.TaskEvent) *quotaNetTaskEventResponse {
	if event == nil {
		return nil
	}
	return &quotaNetTaskEventResponse{
		ID:        event.ID,
		TaskID:    event.TaskID,
		EventType: event.EventType,
		Sequence:  event.Sequence,
		Payload:   event.Payload,
		CreatedAt: formatQuotaNetTime(event.CreatedAt),
	}
}

func quotaNetOptionalTime(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	v := formatQuotaNetTime(*t)
	return &v
}
