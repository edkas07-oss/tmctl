package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzEndpoint(t *testing.T) {
	srv := NewServer(ServerConfig{
		Host: "localhost",
		Port: 8099,
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddleware(t *testing.T) {
	srv := NewServer(ServerConfig{
		Host:   "localhost",
		Port:   8099,
		APIKey: "secret-test-token",
	})

	handler := srv.Handler()

	// 1. Without auth
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stack/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}

	// 2. With X-API-Key
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/stack/status", nil)
	reqAuth.Header.Set("X-API-Key", "secret-test-token")
	wAuth := httptest.NewRecorder()
	handler.ServeHTTP(wAuth, reqAuth)
	if wAuth.Code == http.StatusUnauthorized {
		t.Errorf("expected authorized request, got 401")
	}
}
