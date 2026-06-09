package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterQuotaNetRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	if h == nil || h.QuotaNet == nil {
		return
	}
	quotanet := v1.Group("/quotanet")
	{
		quotanet.POST("/nodes/register", h.QuotaNet.RegisterNode)
		quotanet.GET("/nodes/ws", h.QuotaNet.NodeWebSocket)
	}
}
