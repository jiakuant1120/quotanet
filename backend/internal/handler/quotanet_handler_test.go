package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/tasks"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func TestQuotaNetOpenAIModelsReturnsRegistryModels(t *testing.T) {
	reg := registry.New()
	if err := reg.Register(registry.Session{
		SessionID:          "sess-1",
		NodeID:             1,
		NodeKey:            "node-1",
		InstanceID:         "inst-1",
		WalletAddress:      "wallet-1",
		Status:             protocol.NodeStatusReady,
		MaxConcurrency:     2,
		CurrentConcurrency: 0,
		LastHeartbeatAt:    time.Now(),
		Capabilities: []protocol.Capability{
			{Provider: "openai", Models: []string{"gpt-4.1", "gpt-5"}},
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	h := &QuotaNetHandler{registry: reg}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/quotanet/openai/v1/models", nil)

	h.OpenAIModels(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if body.Object != "list" || len(body.Data) != 2 || body.Data[0].ID != "gpt-4.1" || body.Data[0].Object != "model" || body.Data[0].OwnedBy != "quotanet" {
		t.Fatalf("body = %+v", body)
	}
}

func TestQuotaNetOpenAIChatCompletionsRequiresModel(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"messages":[]}`)
	h := &QuotaNetHandler{taskService: &stubQuotaNetTaskService{}}

	h.OpenAIChatCompletions(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestQuotaNetOpenAIChatCompletionsNoNode(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"model":"gpt-4.1","messages":[]}`)
	h := &QuotaNetHandler{taskService: &stubQuotaNetTaskService{err: tasks.ErrNoNodeAvailable}}

	h.OpenAIChatCompletions(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body=%s", w.Code, w.Body.String())
	}
}

func TestQuotaNetOpenAIChatCompletionsReturnsPayload(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"model":"gpt-4.1","messages":[],"timeout_seconds":10}`)
	h := &QuotaNetHandler{taskService: &stubQuotaNetTaskService{
		result: &tasks.DispatchResult{
			Response: protocol.TaskResponse{
				TaskID:  "task-1",
				Status:  protocol.TaskStatusSuccess,
				Payload: map[string]any{"id": "chatcmpl-1", "object": "chat.completion"},
			},
		},
	}}

	h.OpenAIChatCompletions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	if h.taskService.(*stubQuotaNetTaskService).input.Model != "gpt-4.1" || h.taskService.(*stubQuotaNetTaskService).input.TimeoutSeconds != 10 {
		t.Fatalf("input = %+v", h.taskService.(*stubQuotaNetTaskService).input)
	}
	if got := w.Body.String(); !bytes.Contains([]byte(got), []byte(`"chatcmpl-1"`)) {
		t.Fatalf("body = %s", got)
	}
}

func TestQuotaNetOpenAIChatCompletionsPropagatesCallerContext(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"model":"gpt-4.1","messages":[]}`)
	groupID := int64(7)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:      11,
		UserID:  22,
		GroupID: &groupID,
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 33})
	h := &QuotaNetHandler{taskService: &stubQuotaNetTaskService{
		result: &tasks.DispatchResult{
			Response: protocol.TaskResponse{
				TaskID:  "task-1",
				Status:  protocol.TaskStatusSuccess,
				Payload: map[string]any{"id": "chatcmpl-1"},
			},
		},
	}}

	h.OpenAIChatCompletions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	input := h.taskService.(*stubQuotaNetTaskService).input
	if input.APIKeyID == nil || *input.APIKeyID != 11 {
		t.Fatalf("api_key_id = %v, want 11", input.APIKeyID)
	}
	if input.UserID == nil || *input.UserID != 33 {
		t.Fatalf("user_id = %v, want authenticated subject user 33", input.UserID)
	}
	if input.GroupID == nil || *input.GroupID != 7 {
		t.Fatalf("group_id = %v, want 7", input.GroupID)
	}
}

func newQuotaNetTestContext(w *httptest.ResponseRecorder, body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/quotanet/openai/v1/chat/completions", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

type stubQuotaNetTaskService struct {
	input  tasks.CreateTaskInput
	result *tasks.DispatchResult
	err    error
}

func (s *stubQuotaNetTaskService) DispatchAndWait(_ context.Context, input tasks.CreateTaskInput) (*tasks.DispatchResult, error) {
	s.input = input
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return nil, errors.New("missing result")
	}
	return s.result, nil
}
