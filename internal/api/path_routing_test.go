package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSingleAllowedGroupMiddleware_AttachesRouteContextForSingleGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("accessMetadata", map[string]string{"allowed-channel-groups": "pro"})
		c.Next()
	})
	engine.Use(singleAllowedChannelGroupMiddleware())
	engine.GET("/v1/models", func(c *gin.Context) {
		route := pathRouteContextFromGin(c)
		if route == nil {
			t.Fatalf("expected path route context, got nil")
		}
		if route.Group != "pro" {
			t.Fatalf("route group = %q, want %q", route.Group, "pro")
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSingleAllowedGroupMiddleware_SkipsWhenMultipleGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("accessMetadata", map[string]string{"allowed-channel-groups": "pro,team-a"})
		c.Next()
	})
	engine.Use(singleAllowedChannelGroupMiddleware())
	engine.GET("/v1/models", func(c *gin.Context) {
		if route := pathRouteContextFromGin(c); route != nil {
			t.Fatalf("expected no path route context, got %#v", route)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSingleAllowedGroupMiddleware_SkipsWithoutAccessMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(singleAllowedChannelGroupMiddleware())
	engine.GET("/v1/models", func(c *gin.Context) {
		if route := pathRouteContextFromGin(c); route != nil {
			t.Fatalf("expected no path route context, got %#v", route)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
