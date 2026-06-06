package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/quotanet/nodes"
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

func TestQuotaNetRegisterNodeDisabled(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"wallet_address":"wallet","protocol_version":"2026-06-qt1"}`)
	h := &QuotaNetHandler{
		nodeManager:         nodes.NewManager(&stubQuotaNetNodeStore{}),
		registrationEnabled: func() bool { return false },
	}

	h.RegisterNode(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", w.Code, w.Body.String())
	}
}

func TestQuotaNetRegisterNodeReturnsNodeKeyAndToken(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"name":"dev","wallet_address":"wallet","protocol_version":"2026-06-qt1","client_version":"v0.1.0"}`)
	h := &QuotaNetHandler{
		nodeManager:         nodes.NewManager(&stubQuotaNetNodeStore{}),
		registrationEnabled: func() bool { return true },
	}

	h.RegisterNode(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Node struct {
			NodeKey       string `json:"node_key"`
			WalletAddress string `json:"wallet_address"`
			Status        string `json:"status"`
		} `json:"node"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if body.Node.NodeKey == "" || !strings.HasPrefix(body.Node.NodeKey, "qnn_") {
		t.Fatalf("node_key = %q, want qnn_ prefix", body.Node.NodeKey)
	}
	if body.Token == "" || !strings.HasPrefix(body.Token, "qnc_") {
		t.Fatalf("token = %q, want qnc_ prefix", body.Token)
	}
	if body.Node.WalletAddress != "wallet" || body.Node.Status != nodes.StatusActive {
		t.Fatalf("node = %+v", body.Node)
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

func TestQuotaNetOpenAIChatCompletionsNormalizesEmptyTaskError(t *testing.T) {
	w := httptest.NewRecorder()
	c := newQuotaNetTestContext(w, `{"model":"gpt-4.1","messages":[]}`)
	h := &QuotaNetHandler{taskService: &stubQuotaNetTaskService{
		result: &tasks.DispatchResult{
			Response: protocol.TaskResponse{
				TaskID: "task-1",
				Status: protocol.TaskStatusTimeout,
			},
		},
	}}

	h.OpenAIChatCompletions(c)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if body.Error.Type != "api_error" || body.Error.Message != "quotanet task timed out" {
		t.Fatalf("error = %+v", body.Error)
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

type stubQuotaNetNodeStore struct {
	node *nodes.Node
}

func (s *stubQuotaNetNodeStore) Create(_ context.Context, input nodes.CreateInput, nodeKey, tokenHash string) (*nodes.Node, error) {
	s.node = &nodes.Node{
		ID:            1,
		NodeKey:       nodeKey,
		Name:          input.Name,
		WalletAddress: input.WalletAddress,
		TokenHash:     tokenHash,
		Status:        input.Status,
	}
	return s.node, nil
}

func (s *stubQuotaNetNodeStore) GetByWalletAddress(_ context.Context, walletAddress string) (*nodes.Node, error) {
	if s.node == nil || s.node.WalletAddress != walletAddress {
		return nil, nodes.ErrNodeNotFound
	}
	return s.node, nil
}

func (s *stubQuotaNetNodeStore) UpdateRegistration(_ context.Context, id int64, input nodes.CreateInput, tokenHash string, updateToken bool) (*nodes.Node, error) {
	if s.node == nil {
		return nil, nodes.ErrNodeNotFound
	}
	s.node.ID = id
	s.node.Name = input.Name
	s.node.WalletAddress = input.WalletAddress
	s.node.Status = input.Status
	if updateToken {
		s.node.TokenHash = tokenHash
	}
	return s.node, nil
}

func (s *stubQuotaNetNodeStore) List(_ context.Context, _ nodes.ListParams) ([]*nodes.Node, int64, error) {
	return nil, 0, nil
}

func (s *stubQuotaNetNodeStore) GetByID(_ context.Context, id int64) (*nodes.Node, error) {
	return &nodes.Node{ID: id}, nil
}

func (s *stubQuotaNetNodeStore) UpdateStatus(_ context.Context, id int64, status string) (*nodes.Node, error) {
	return &nodes.Node{ID: id, Status: status}, nil
}

func (s *stubQuotaNetNodeStore) ResetToken(_ context.Context, id int64, tokenHash string) (*nodes.Node, error) {
	return &nodes.Node{ID: id, TokenHash: tokenHash}, nil
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
