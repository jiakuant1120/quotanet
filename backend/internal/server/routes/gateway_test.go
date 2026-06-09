package routes

import (
	"context"
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

func newQuotaNetFallbackGatewayRoutesTestRouter(total, active int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
			QuotaNet:      handler.NewQuotaNetHandler(nil, nil, nil, stubQuotaNetGroupCounter{total: total, active: active}),
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

type stubQuotaNetGroupCounter struct {
	total  int64
	active int64
}

func (s stubQuotaNetGroupCounter) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return s.total, s.active, nil
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

func TestGatewayRoutesOpenAIEmptyGroupResponsesPathsRouteToQuotaNet(t *testing.T) {
	router := newQuotaNetFallbackGatewayRoutesTestRouter(0, 0)

	for _, path := range []string{
		"/v1/responses",
		"/v1/responses/compact",
		"/responses",
		"/responses/compact",
		"/backend-api/codex/responses",
		"/backend-api/codex/responses/compact",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5","input":"ping"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("path=%s status=%d want %d body=%s", path, w.Code, http.StatusServiceUnavailable, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "quotanet task service is not initialized") {
			t.Fatalf("path=%s body=%s should contain quotanet task service error", path, w.Body.String())
		}
	}
}

func TestGatewayRoutesOpenAIEmptyGroupChatCompletionsPathsRouteToQuotaNet(t *testing.T) {
	router := newQuotaNetFallbackGatewayRoutesTestRouter(0, 0)

	for _, path := range []string{
		"/v1/chat/completions",
		"/chat/completions",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"ping"}]}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("path=%s status=%d want %d body=%s", path, w.Code, http.StatusServiceUnavailable, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "quotanet task service is not initialized") {
			t.Fatalf("path=%s body=%s should contain quotanet task service error", path, w.Body.String())
		}
	}
}

func TestGatewayRoutesOpenAIEmptyGroupResponsesWebSocketIsExplicitlyUnsupported(t *testing.T) {
	router := newQuotaNetFallbackGatewayRoutesTestRouter(0, 0)

	for _, path := range []string{
		"/v1/responses",
		"/responses",
		"/backend-api/codex/responses",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Fatalf("path=%s status=%d want %d body=%s", path, w.Code, http.StatusNotImplemented, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Responses WebSocket ingress") {
			t.Fatalf("path=%s body=%s should explain unsupported quotanet websocket ingress", path, w.Body.String())
		}
	}
}

func TestGatewayRoutesOpenAIGroupWithOnlyUnavailableAccountsDoesNotRouteToQuotaNet(t *testing.T) {
	router := newQuotaNetFallbackGatewayRoutesTestRouter(1, 0)

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "quotanet task service is not initialized") {
		t.Fatalf("body=%s should not route to quotanet when total accounts > 0", w.Body.String())
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
