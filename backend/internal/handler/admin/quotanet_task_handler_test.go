package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQuotaNetTaskListParamsParsesCallerFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?api_key_id=11&group_id=22&user_id=33", nil)

	params, ok := quotaNetTaskListParams(c, 1, 20)
	if !ok {
		t.Fatal("quotaNetTaskListParams() ok = false")
	}
	if params.APIKeyID == nil || *params.APIKeyID != 11 {
		t.Fatalf("api_key_id = %v, want 11", params.APIKeyID)
	}
	if params.GroupID == nil || *params.GroupID != 22 {
		t.Fatalf("group_id = %v, want 22", params.GroupID)
	}
	if params.UserID == nil || *params.UserID != 33 {
		t.Fatalf("user_id = %v, want 33", params.UserID)
	}
}

func TestQuotaNetTaskDispatchInputParsesNodeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{
		"request_id": "req-1",
		"node_id": 42,
		"platform": "openai",
		"endpoint": "/v1/chat/completions",
		"model": "gpt-4.1",
		"payload": {"messages": []}
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	input, ok := quotaNetTaskDispatchInput(c)
	if !ok {
		t.Fatal("quotaNetTaskDispatchInput() ok = false")
	}
	if input.NodeID == nil || *input.NodeID != 42 {
		t.Fatalf("node_id = %v, want 42", input.NodeID)
	}
}

func TestQuotaNetTaskListParamsRejectsInvalidCallerFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?api_key_id=bad", nil)

	_, ok := quotaNetTaskListParams(c, 1, 20)
	if ok {
		t.Fatal("quotaNetTaskListParams() ok = true, want false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
