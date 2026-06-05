package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	qws "github.com/Wei-Shaw/sub2api/internal/quotanet/ws"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	quotanetWriteTimeout = 10 * time.Second
)

type QuotaNetHandler struct {
	sessionManager *qws.SessionManager
	upgrader       websocket.Upgrader
}

func NewQuotaNetHandler(sessionManager *qws.SessionManager) *QuotaNetHandler {
	return &QuotaNetHandler{
		sessionManager: sessionManager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			},
		},
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
	if err := h.sessionManager.Serve(c.Request.Context(), &gorillaConn{conn: conn}, qws.ServeOptions{
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
