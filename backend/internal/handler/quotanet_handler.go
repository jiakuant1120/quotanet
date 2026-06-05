package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/protocol"
	"github.com/Wei-Shaw/sub2api/internal/quotanet/tasks"
	qws "github.com/Wei-Shaw/sub2api/internal/quotanet/ws"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	quotanetWriteTimeout = 10 * time.Second
)

type QuotaNetHandler struct {
	sessionManager *qws.SessionManager
	taskService    quotaNetTaskService
	upgrader       websocket.Upgrader
}

type quotaNetTaskService interface {
	DispatchAndWait(ctx context.Context, input tasks.CreateTaskInput) (*tasks.DispatchResult, error)
}

func NewQuotaNetHandler(sessionManager *qws.SessionManager, taskService *tasks.Service) *QuotaNetHandler {
	return &QuotaNetHandler{
		sessionManager: sessionManager,
		taskService:    taskService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			},
		},
	}
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
		c.JSON(status, gin.H{"error": gin.H{
			"type":    strings.TrimSpace(result.Response.ErrorCode),
			"message": strings.TrimSpace(result.Response.ErrorMessage),
		}})
		return
	}
	if result.Response.Payload == nil {
		result.Response.Payload = map[string]any{}
	}
	c.JSON(http.StatusOK, result.Response.Payload)
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
