package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func newGatewayRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: service.PlatformOpenAI},
			})
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
	)

	return router
}

func TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/responses/compact",
		"/responses/compact",
		"/backend-api/codex/responses",
		"/backend-api/codex/responses/compact",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Fatalf("path=%s should hit OpenAI responses handler", path)
		}
	}
}

func TestGatewayRoutesOpenAIImagesPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/images/generations",
		"/v1/images/edits",
		"/images/generations",
		"/images/edits",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Fatalf("path=%s should hit OpenAI images handler", path)
		}
	}
}

func TestQuotaNetOpenAIRouteRequiresAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterQuotaNetGatewayRoutes(
		v1,
		&handler.Handlers{QuotaNet: handler.NewQuotaNetHandler(nil, nil)},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "api key required"})
			c.Abort()
		}),
		nil,
		nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotanet/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4.1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "api key required") {
		t.Fatalf("body=%s should contain api key required", w.Body.String())
	}
}

func TestQuotaNetOpenAIRouteUsesAuthenticatedOpenAIGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterQuotaNetGatewayRoutes(
		v1,
		&handler.Handlers{QuotaNet: handler.NewQuotaNetHandler(nil, nil)},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: service.PlatformOpenAI},
			})
			c.Next()
		}),
		nil,
		nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotanet/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4.1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid quotanet task input") {
		t.Fatalf("body=%s should contain quotanet handler error", w.Body.String())
	}
}

func TestQuotaNetOpenAIModelsRouteUsesAuthenticatedOpenAIGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterQuotaNetGatewayRoutes(
		v1,
		&handler.Handlers{QuotaNet: handler.NewQuotaNetHandler(nil, nil)},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: service.PlatformOpenAI},
			})
			c.Next()
		}),
		nil,
		nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/quotanet/openai/v1/models", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want %d body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "quotanet registry is not initialized") {
		t.Fatalf("body=%s should contain registry unavailable message", w.Body.String())
	}
}
