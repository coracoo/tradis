package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dockerpanel/server/backend/handlers"
	"github.com/gin-gonic/gin"
)

func TestIPAllowlistMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	allow, _ := handlers.NewIPAllowlist("127.0.0.1")

	r := gin.New()
	r.Use(ipAllowlistMiddleware(allow))
	r.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })

	req1 := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != 200 {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w2.Code)
	}
}
