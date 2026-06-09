package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/nodes"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/registry"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/tasks"
	qws "github.com/Wei-Shaw/sub2api/internal/quotanet/ws"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

const (
	quotanetWriteTimeout    = 10 * time.Second
	quotanetModelStaleAfter = 60 * time.Second
)

type QuotaNetHandler struct {
	sessionManager      *qws.SessionManager
	nodeManager         *nodes.Manager
	taskService         quotaNetTaskService
	registry            *registry.Registry
	upgrader            websocket.Upgrader
	registrationEnabled func() bool
}

type quotaNetTaskService interface {
	DispatchAndWait(ctx context.Context, input tasks.CreateTaskInput) (*tasks.DispatchResult, error)
}

func NewQuotaNetHandler(sessionManager *qws.SessionManager, nodeManager *nodes.Manager, taskService *tasks.Service) *QuotaNetHandler {
	var reg *registry.Registry
	if sessionManager != nil {
		reg = sessionManager.Registry()
	}
	var taskSvc quotaNetTaskService
	if taskService != nil {
		taskSvc = taskService
	}
	return &QuotaNetHandler{
		sessionManager: sessionManager,
		nodeManager:    nodeManager,
		taskService:    taskSvc,
		registry:       reg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			},
		},
		registrationEnabled: quotanetDevelopmentRegistrationEnabled,
	}
}

type quotaNetNodeRegisterRequest struct {
	Name            string `json:"name"`
	WalletAddress   string `json:"wallet_address"`
	ClientVersion   string `json:"client_version"`
	ProtocolVersion string `json:"protocol_version"`
	ResetToken      bool   `json:"reset_token"`
}

type quotaNetNodeRegisterNodeResponse struct {
	ID            int64  `json:"id"`
	NodeKey       string `json:"node_key"`
	Name          string `json:"name"`
	WalletAddress string `json:"wallet_address"`
	Status        string `json:"status"`
}

type quotaNetNodeRegisterResponse struct {
	Node  quotaNetNodeRegisterNodeResponse `json:"node"`
	Token string                           `json:"token,omitempty"`
}

func (h *QuotaNetHandler) RegisterNode(c *gin.Context) {
	if h == nil || h.nodeManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quotanet node manager is not initialized"})
		return
	}
	enabled := quotanetDevelopmentRegistrationEnabled
	if h.registrationEnabled != nil {
		enabled = h.registrationEnabled
	}
	if !enabled() {
		c.JSON(http.StatusForbidden, gin.H{"error": "quotanet node registration is disabled"})
		return
	}
	var req quotaNetNodeRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	result, err := h.nodeManager.RegisterDevelopmentNode(c.Request.Context(), nodes.RegisterDevelopmentNodeInput{
		Name:            req.Name,
		WalletAddress:   req.WalletAddress,
		ClientVersion:   req.ClientVersion,
		ProtocolVersion: req.ProtocolVersion,
		ResetToken:      req.ResetToken,
		AllowResetToken: quotanetRegistrationResetAllowed(),
	})
	if err != nil {
		quotanetNodeRegisterError(c, err, req.ProtocolVersion)
		return
	}
	c.JSON(http.StatusOK, quotaNetNodeRegisterResponse{
		Node:  quotaNetNodeRegisterNode(result.Node),
		Token: result.Token,
	})
}

func (h *QuotaNetHandler) OpenAIModels(c *gin.Context) {
	h.Models(c)
}

func (h *QuotaNetHandler) Models(c *gin.Context) {
	if h == nil || h.registry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"type":    "api_error",
			"message": "quotanet registry is not initialized",
		}})
		return
	}
	modelIDs := h.registry.AvailableModels("openai", quotanetModelStaleAfter)
	models := make([]gin.H, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		models = append(models, gin.H{
			"id":       modelID,
			"object":   "model",
			"created":  0,
			"owned_by": "quotanet",
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}

func (h *QuotaNetHandler) Responses(c *gin.Context) {
	if h == nil || h.taskService == nil {
		quotanetResponsesError(c, http.StatusServiceUnavailable, "api_error", "quotanet task service is not initialized")
		return
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		quotanetResponsesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		quotanetResponsesError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	if !gjson.ValidBytes(body) {
		quotanetResponsesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || strings.TrimSpace(modelResult.String()) == "" {
		quotanetResponsesError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		quotanetResponsesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	model := strings.TrimSpace(modelResult.String())
	stream := gjson.GetBytes(body, "stream").Bool()
	input := tasks.CreateTaskInput{
		RequestID:      quotanetRequestID(c),
		Platform:       "openai",
		Endpoint:       quotanetRequestEndpoint(c, "/responses"),
		Model:          model,
		Stream:         stream,
		TimeoutSeconds: quotanetTimeoutSeconds(payload),
		Payload:        payload,
	}
	applyQuotaNetCallerContext(c, &input)
	result, err := h.taskService.DispatchAndWait(c.Request.Context(), input)
	if err != nil {
		quotanetResponsesDispatchError(c, err)
		return
	}
	if result.Response.Status != protocol.TaskStatusSuccess {
		quotanetResponsesTaskError(c, result.Response)
		return
	}
	if result.Response.Payload == nil {
		result.Response.Payload = map[string]any{}
	}
	responsePayload := quotanetResponsesObjectFromPayload(result.Response.Payload, model, result.Response.TaskID)
	quotanetNormalizeResponsesCompletedResponse(responsePayload, model, result.Response.TaskID)
	if stream {
		quotanetWriteResponsesCompletedSSE(c, responsePayload, model, result.Response.TaskID)
		return
	}
	c.JSON(http.StatusOK, responsePayload)
}

func (h *QuotaNetHandler) OpenAIChatCompletions(c *gin.Context) {
	if h == nil || h.taskService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"type":    "api_error",
			"message": "quotanet task service is not initialized",
		}})
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"type":    "invalid_request_error",
			"message": "invalid JSON request body",
		}})
		return
	}
	model, _ := payload["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"type":    "invalid_request_error",
			"message": "model is required",
		}})
		return
	}
	stream, _ := payload["stream"].(bool)
	input := tasks.CreateTaskInput{
		RequestID:      quotanetRequestID(c),
		Platform:       "openai",
		Endpoint:       "/v1/chat/completions",
		Model:          model,
		Stream:         stream,
		TimeoutSeconds: quotanetTimeoutSeconds(payload),
		Payload:        payload,
	}
	applyQuotaNetCallerContext(c, &input)
	result, err := h.taskService.DispatchAndWait(c.Request.Context(), input)
	if err != nil {
		quotanetOpenAIError(c, err)
		return
	}
	if result.Response.Status != protocol.TaskStatusSuccess {
		status := http.StatusBadGateway
		if result.Response.Status == protocol.TaskStatusTimeout {
			status = http.StatusGatewayTimeout
		}
		c.JSON(status, gin.H{"error": quotanetTaskOpenAIErrorPayload(result.Response)})
		return
	}
	if result.Response.Payload == nil {
		result.Response.Payload = map[string]any{}
	}
	c.JSON(http.StatusOK, result.Response.Payload)
}

func quotanetTaskOpenAIErrorPayload(resp protocol.TaskResponse) gin.H {
	errorType := strings.TrimSpace(resp.ErrorCode)
	if errorType == "" {
		errorType = "api_error"
	}
	message := strings.TrimSpace(resp.ErrorMessage)
	if message == "" {
		switch resp.Status {
		case protocol.TaskStatusTimeout:
			message = "quotanet task timed out"
		case protocol.TaskStatusCancelled:
			message = "quotanet task was cancelled"
		default:
			message = "quotanet task failed"
		}
	}
	return gin.H{
		"type":    errorType,
		"message": message,
	}
}

func applyQuotaNetCallerContext(c *gin.Context, input *tasks.CreateTaskInput) {
	if c == nil || input == nil {
		return
	}
	if apiKey, ok := middleware.GetAPIKeyFromContext(c); ok && apiKey != nil {
		input.APIKeyID = &apiKey.ID
		if apiKey.UserID > 0 {
			input.UserID = &apiKey.UserID
		}
		input.GroupID = apiKey.GroupID
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		input.UserID = &subject.UserID
	}
}

func (h *QuotaNetHandler) NodeWebSocket(c *gin.Context) {
	if h == nil || h.sessionManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quotanet service is not initialized"})
		return
	}
	token := quotanetNodeToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "quotanet node token is required"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	sessionID := "qns_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	instanceID := quotanetInstanceID(c)
	if instanceID == "" {
		instanceID = sessionID
	}
	reason := "client_ip=" + ip.GetClientIP(c)
	if err := h.sessionManager.Serve(c.Request.Context(), qws.NewSerialConn(&gorillaConn{conn: conn}), qws.ServeOptions{
		SessionID:   sessionID,
		InstanceID:  instanceID,
		Token:       token,
		RemoteAddr:  ip.GetClientIP(c),
		CloseReason: reason,
	}); err != nil {
		return
	}
}

func quotanetNodeToken(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if token := strings.TrimSpace(c.GetHeader("X-QuotaNet-Token")); token != "" {
		return token
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return strings.TrimSpace(c.Query("token"))
}

func quotanetInstanceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id := strings.TrimSpace(c.GetHeader("X-QuotaNet-Instance-ID")); id != "" {
		return id
	}
	return strings.TrimSpace(c.Query("instance_id"))
}

func quotanetRequestID(c *gin.Context) string {
	if c != nil {
		if requestID := strings.TrimSpace(c.GetHeader("X-Request-ID")); requestID != "" {
			return requestID
		}
	}
	return "http_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func quotanetTimeoutSeconds(payload map[string]any) int {
	switch v := payload["timeout_seconds"].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return 60
}

func quotanetRequestEndpoint(c *gin.Context, fallback string) string {
	if c == nil || c.Request == nil {
		return fallback
	}
	path := strings.TrimSpace(c.Request.URL.Path)
	if path == "" {
		return fallback
	}
	return path
}

func quotanetResponsesDispatchError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "quotanet task operation failed"
	switch {
	case errors.Is(err, tasks.ErrInvalidTaskInput):
		status = http.StatusBadRequest
		message = "invalid quotanet task input"
	case errors.Is(err, tasks.ErrNoNodeAvailable):
		status = http.StatusServiceUnavailable
		message = "No available QuotaNet nodes"
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
		message = "quotanet task timed out"
	}
	quotanetResponsesError(c, status, "api_error", message)
}

func quotanetResponsesTaskError(c *gin.Context, resp protocol.TaskResponse) {
	status := http.StatusBadGateway
	if resp.Status == protocol.TaskStatusTimeout {
		status = http.StatusGatewayTimeout
	}
	code := strings.TrimSpace(resp.ErrorCode)
	if code == "" {
		code = "api_error"
	}
	message := strings.TrimSpace(resp.ErrorMessage)
	if message == "" {
		switch resp.Status {
		case protocol.TaskStatusTimeout:
			message = "quotanet task timed out"
		case protocol.TaskStatusCancelled:
			message = "quotanet task was cancelled"
		default:
			message = "quotanet task failed"
		}
	}
	quotanetResponsesError(c, status, code, message)
}

func quotanetResponsesError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{
		"code":    code,
		"message": message,
	}})
}

func quotanetWriteResponsesCompletedSSE(c *gin.Context, payload map[string]any, model, taskID string) {
	eventPayload := quotanetResponsesCompletedEvent(payload, model, taskID)
	data, err := json.Marshal(eventPayload)
	if err != nil {
		quotanetResponsesError(c, http.StatusInternalServerError, "api_error", "Failed to encode streaming response")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	if err := quotanetWriteResponsesTextLifecycleSSE(c, eventPayload); err != nil {
		_ = c.Error(err)
		return
	}
	if _, err := fmt.Fprintf(c.Writer, "event: response.completed\ndata: %s\n\n", data); err != nil {
		_ = c.Error(err)
		return
	}
	if _, err := fmt.Fprint(c.Writer, "data: [DONE]\n\n"); err != nil {
		_ = c.Error(err)
		return
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func quotanetResponsesCompletedEvent(payload map[string]any, model, taskID string) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	if typ, _ := payload["type"].(string); typ == "response.completed" {
		if response, ok := payload["response"].(map[string]any); ok {
			if response["usage"] == nil && payload["usage"] != nil {
				response["usage"] = payload["usage"]
			}
			quotanetNormalizeResponsesCompletedResponse(response, model, taskID)
			return payload
		}
	}
	response := quotanetResponsesObjectFromPayload(payload, model, taskID)
	quotanetNormalizeResponsesCompletedResponse(response, model, taskID)
	return map[string]any{
		"type":     "response.completed",
		"response": response,
	}
}

func quotanetResponsesObjectFromPayload(payload map[string]any, model, taskID string) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	if strings.EqualFold(strings.TrimSpace(asString(payload["object"])), "chat.completion") {
		responseID := firstNonEmpty(asString(payload["id"]), quotanetResponseID(taskID))
		response := map[string]any{
			"id":     responseID,
			"object": "response",
			"status": "completed",
			"model":  firstNonEmpty(asString(payload["model"]), strings.TrimSpace(model)),
			"output": quotanetResponseOutputFromText(quotanetChatCompletionText(payload), quotanetMessageItemID(responseID)),
			"usage":  payload["usage"],
		}
		if created, ok := payload["created"]; ok {
			response["created_at"] = created
		}
		return response
	}
	return payload
}

func quotanetNormalizeResponsesCompletedResponse(response map[string]any, model, taskID string) {
	if strings.TrimSpace(asString(response["id"])) == "" {
		response["id"] = quotanetResponseID(taskID)
	}
	response["object"] = "response"
	if strings.TrimSpace(asString(response["status"])) == "" {
		response["status"] = "completed"
	}
	if strings.TrimSpace(asString(response["model"])) == "" && strings.TrimSpace(model) != "" {
		response["model"] = strings.TrimSpace(model)
	}
	if output, ok := response["output"]; !ok || quotanetOutputIsEmpty(output) {
		if text := quotanetChatCompletionText(response); strings.TrimSpace(text) != "" {
			response["output"] = quotanetResponseOutputFromText(text, quotanetMessageItemID(asString(response["id"])))
		} else {
			response["output"] = []any{}
		}
	}
	if _, ok := response["output"]; !ok {
		response["output"] = []any{}
	}
	response["usage"] = quotanetNormalizeResponsesUsage(response["usage"])
}

func quotanetWriteResponsesTextLifecycleSSE(c *gin.Context, completedEvent map[string]any) error {
	response, _ := completedEvent["response"].(map[string]any)
	if response == nil {
		return nil
	}
	text := quotanetResponsesOutputText(response["output"])
	if text == "" {
		return nil
	}
	responseID := firstNonEmpty(asString(response["id"]), quotanetResponseID(""))
	model := asString(response["model"])
	itemID := quotanetMessageItemID(responseID)
	events := []map[string]any{
		{
			"type": "response.created",
			"response": map[string]any{
				"id":     responseID,
				"object": "response",
				"model":  model,
				"status": "in_progress",
				"output": []any{},
			},
		},
		{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item": map[string]any{
				"type":    "message",
				"id":      itemID,
				"role":    "assistant",
				"status":  "in_progress",
				"content": []any{map[string]any{"type": "output_text", "text": ""}},
			},
		},
		{
			"type":          "response.content_part.added",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"part":          map[string]any{"type": "output_text", "text": ""},
		},
		{
			"type":          "response.output_text.delta",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"delta":         text,
		},
		{
			"type":          "response.output_text.done",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"text":          text,
		},
		{
			"type":          "response.content_part.done",
			"output_index":  0,
			"content_index": 0,
			"item_id":       itemID,
			"part":          map[string]any{"type": "output_text", "text": text},
		},
		{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item": map[string]any{
				"type":    "message",
				"id":      itemID,
				"role":    "assistant",
				"status":  "completed",
				"content": []any{map[string]any{"type": "output_text", "text": text}},
			},
		},
	}
	for _, event := range events {
		eventType := asString(event["type"])
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, data); err != nil {
			return err
		}
	}
	return nil
}

func quotanetChatCompletionText(payload map[string]any) string {
	choices, _ := payload["choices"].([]any)
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]any)
	if choice == nil {
		return ""
	}
	message, _ := choice["message"].(map[string]any)
	if message == nil {
		return ""
	}
	return asString(message["content"])
}

func quotanetResponseOutputFromText(text, itemID string) []any {
	if text == "" {
		return []any{}
	}
	if strings.TrimSpace(itemID) == "" {
		itemID = "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return []any{map[string]any{
		"type":   "message",
		"id":     itemID,
		"role":   "assistant",
		"status": "completed",
		"content": []any{map[string]any{
			"type": "output_text",
			"text": text,
		}},
	}}
}

func quotanetResponsesOutputText(output any) string {
	items, _ := output.([]any)
	for _, rawItem := range items {
		item, _ := rawItem.(map[string]any)
		if item == nil || asString(item["type"]) != "message" {
			continue
		}
		parts, _ := item["content"].([]any)
		var b strings.Builder
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]any)
			if part == nil || asString(part["type"]) != "output_text" {
				continue
			}
			b.WriteString(asString(part["text"]))
		}
		if b.Len() > 0 {
			return b.String()
		}
	}
	return ""
}

func quotanetOutputIsEmpty(output any) bool {
	items, ok := output.([]any)
	return !ok || len(items) == 0
}

func quotanetMessageItemID(responseID string) string {
	responseID = strings.TrimSpace(responseID)
	responseID = strings.TrimPrefix(responseID, "resp_")
	responseID = strings.ReplaceAll(responseID, "-", "")
	if responseID == "" {
		responseID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return "msg_" + responseID
}

func quotanetNormalizeResponsesUsage(raw any) map[string]any {
	usage, _ := raw.(map[string]any)
	if usage == nil {
		usage = map[string]any{}
	}
	inputTokens, ok := quotanetIntValue(usage["input_tokens"])
	if !ok {
		inputTokens, _ = quotanetIntValue(usage["prompt_tokens"])
	}
	outputTokens, ok := quotanetIntValue(usage["output_tokens"])
	if !ok {
		outputTokens, _ = quotanetIntValue(usage["completion_tokens"])
	}
	totalTokens, ok := quotanetIntValue(usage["total_tokens"])
	if !ok {
		totalTokens = inputTokens + outputTokens
	}
	usage["input_tokens"] = inputTokens
	usage["output_tokens"] = outputTokens
	usage["total_tokens"] = totalTokens
	delete(usage, "prompt_tokens")
	delete(usage, "completion_tokens")
	return usage
}

func quotanetIntValue(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func quotanetResponseID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	taskID = strings.TrimPrefix(taskID, "qnt_")
	taskID = strings.TrimPrefix(taskID, "resp_")
	return "resp_" + strings.ReplaceAll(taskID, "-", "")
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func quotanetOpenAIError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "quotanet task operation failed"
	switch {
	case errors.Is(err, tasks.ErrInvalidTaskInput):
		status = http.StatusBadRequest
		message = "invalid quotanet task input"
	case errors.Is(err, tasks.ErrNoNodeAvailable):
		status = http.StatusServiceUnavailable
		message = "no quotanet node available"
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
		message = "quotanet task timed out"
	}
	c.JSON(status, gin.H{"error": gin.H{
		"type":    "api_error",
		"message": message,
	}})
}

func quotanetNodeRegisterError(c *gin.Context, err error, clientProtocolVersion string) {
	switch {
	case errors.Is(err, nodes.ErrInvalidNodeInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, protocol.ErrUnsupportedVersion):
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported protocol version: client=" + strings.TrimSpace(clientProtocolVersion) + " server=" + protocol.Version})
	case errors.Is(err, nodes.ErrNodeInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": "quotanet node is disabled"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "quotanet node registration failed"})
	}
}

func quotaNetNodeRegisterNode(node *nodes.Node) quotaNetNodeRegisterNodeResponse {
	if node == nil {
		return quotaNetNodeRegisterNodeResponse{}
	}
	return quotaNetNodeRegisterNodeResponse{
		ID:            node.ID,
		NodeKey:       node.NodeKey,
		Name:          node.Name,
		WalletAddress: node.WalletAddress,
		Status:        node.Status,
	}
}

func quotanetDevelopmentRegistrationEnabled() bool {
	if raw := strings.TrimSpace(os.Getenv("QUOTANET_NODE_REGISTRATION_ENABLED")); raw != "" {
		return parseQuotaNetBool(raw)
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("SERVER_MODE")), "debug")
}

func quotanetRegistrationResetAllowed() bool {
	if raw := strings.TrimSpace(os.Getenv("QUOTANET_NODE_REGISTRATION_ALLOW_RESET_TOKEN")); raw != "" {
		return parseQuotaNetBool(raw)
	}
	return true
}

func parseQuotaNetBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

type gorillaConn struct {
	conn *websocket.Conn
}

func (c *gorillaConn) ReadJSON(v any) error {
	return c.conn.ReadJSON(v)
}

func (c *gorillaConn) WriteJSON(v any) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(quotanetWriteTimeout)); err != nil {
		return err
	}
	return c.conn.WriteJSON(v)
}

func (c *gorillaConn) Close() error {
	return c.conn.Close()
}
