package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

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

func RegisterQuotaNetGatewayRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	opsService *service.OpsService,
	settingService *service.SettingService,
	cfg *config.Config,
) {
	if h == nil || h.QuotaNet == nil {
		return
	}

	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)

	quotanetOpenAI := v1.Group("/quotanet/openai/v1")
	quotanetOpenAI.Use(bodyLimit)
	quotanetOpenAI.Use(clientRequestID)
	quotanetOpenAI.Use(opsErrorLogger)
	quotanetOpenAI.Use(endpointNorm)
	quotanetOpenAI.Use(gin.HandlerFunc(apiKeyAuth))
	quotanetOpenAI.Use(requireGroupAnthropic)
	{
		quotanetOpenAI.GET("/models", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformOpenAI {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "QuotaNet OpenAI models requires an OpenAI group",
					},
				})
				return
			}
			h.QuotaNet.OpenAIModels(c)
		})
		quotanetOpenAI.POST("/chat/completions", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformOpenAI {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "QuotaNet OpenAI chat completions requires an OpenAI group",
					},
				})
				return
			}
			h.QuotaNet.OpenAIChatCompletions(c)
		})
	}
}
