package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/tasks"

	"github.com/gin-gonic/gin"
)

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
