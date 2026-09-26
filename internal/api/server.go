package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eddywiyatno/tmctl/internal/agent"
	"github.com/eddywiyatno/tmctl/internal/buildinfo"
	"github.com/eddywiyatno/tmctl/internal/config"
	"github.com/eddywiyatno/tmctl/internal/diagnostic"
	"github.com/eddywiyatno/tmctl/internal/engine"
	"github.com/eddywiyatno/tmctl/internal/gitops"
	"github.com/eddywiyatno/tmctl/pkg/termutil"
)

// ServerConfig holds the REST API daemon configuration.
type ServerConfig struct {
	Host   string
	Port   int
	APIKey string
}

// Server encapsulates the embedded REST API daemon.
type Server struct {
	cfg        ServerConfig
	mux        *http.ServeMux
	httpServer *http.Server
}

// NewServer initializes the REST API server with routes and middleware.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Port <= 0 {
		cfg.Port = 8099
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	s := &Server{
		cfg: cfg,
		mux: http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler wrapped with middleware.
func (s *Server) Handler() http.Handler {
	handler := http.Handler(s.mux)
	handler = AuthMiddleware(s.cfg.APIKey, handler)
	handler = CORSMiddleware(handler)
	handler = LoggingMiddleware(handler)
	return handler
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) registerRoutes() {
	// Base health check
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "OK",
			"service": "tmctl-api-daemon",
			"version": buildinfo.Version,
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// GitOps Reconciler status
	s.mux.HandleFunc("GET /api/v1/gitops/status", func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("dir")
		spec := r.URL.Query().Get("spec")
		export, err := gitops.StatusGitOpsWithExport(dir, spec)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, export)
	})

	// GitOps trigger sync
	s.mux.HandleFunc("POST /api/v1/gitops/sync", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SpecFile string `json:"spec_file"`
			WorkDir  string `json:"work_dir"`
			Force    bool   `json:"force"`
			DryRun   bool   `json:"dry_run"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		err := gitops.SyncGitOps(gitops.SyncOptions{
			SpecFile: req.SpecFile,
			WorkDir:  req.WorkDir,
			Force:    req.Force,
			DryRun:   req.DryRun,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "Sync executed successfully"})
	})

	// Diagnostic Triage
	s.mux.HandleFunc("POST /api/v1/diagnostic/triage", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TargetContainer string `json:"target_container"`
			SpoolDir        string `json:"spool_dir"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		engineClient, _ := engine.NewEngineClient("auto", "")
		res, err := diagnostic.ExecuteTriage(r.Context(), engineClient, req.TargetContainer, req.SpoolDir, "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	// Agent status
	s.mux.HandleFunc("GET /api/v1/agent/status", func(w http.ResponseWriter, r *http.Request) {
		cfg := agent.DefaultConfig()
		if target := r.URL.Query().Get("target"); target != "" {
			cfg.TargetContainer = target
		}
		if spool := r.URL.Query().Get("spool"); spool != "" {
			cfg.SpoolDir = spool
		}

		err := agent.ShowAgentStatus(cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "checked"})
	})

	// Stack containers status
	s.mux.HandleFunc("GET /api/v1/stack/status", func(w http.ResponseWriter, r *http.Request) {
		engineClient, err := engine.NewEngineClient("auto", "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		containers, err := engineClient.ListContainers(r.Context(), true)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, containers)
	})
}

// Start launches the HTTP server and blocks until SIGINT/SIGTERM.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	handler := http.Handler(s.mux)
	handler = AuthMiddleware(s.cfg.APIKey, handler)
	handler = CORSMiddleware(handler)
	handler = LoggingMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	termutil.Header("Starting tmctl Embedded REST API Daemon")
	termutil.Info("Listening Address : http://%s", addr)
	if s.cfg.APIKey != "" {
		termutil.Info("Authentication    : API Key Enabled (Bearer / X-API-Key)")
	} else {
		termutil.Warn("Authentication    : Disabled (Public Local Endpoint)")
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-sigCh:
		termutil.Info("Received shutdown signal, terminating server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
}

// ServeAPI is the entrypoint for 'tmctl serve'.
func ServeAPI(host string, port int, apiKey string) error {
	_ = config.DefaultConfig()
	srv := NewServer(ServerConfig{
		Host:   host,
		Port:   port,
		APIKey: apiKey,
	})
	return srv.Start()
}
