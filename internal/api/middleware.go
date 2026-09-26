package api

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// LoggingMiddleware logs incoming HTTP requests.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[API] %s %s from %s in %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// CORSMiddleware sets permissive CORS headers for local/portal dashboards.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware verifies API key header or Bearer token if configured.
func AuthMiddleware(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		clientKey := r.Header.Get("X-API-Key")
		if clientKey == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				clientKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if clientKey != apiKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized: invalid or missing API key"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
