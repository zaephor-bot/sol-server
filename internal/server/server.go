// Package server exposes the sol-server HTTP endpoints behind an
// *http.Server that drains gracefully.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Server serves the sol-server HTTP API on a single port.
type Server struct {
	srv *http.Server
}

// New returns a Server that listens on the given TCP port and serves
// the default routes (see Routes).
func New(port int) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           Routes(),
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Addr reports the address the server listens on (":<port>").
func (s *Server) Addr() string { return s.srv.Addr }

// ListenAndServe starts the server and blocks until it is shut down or
// fails. It returns http.ErrServerClosed after a successful Shutdown.
func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the server: it closes the listeners so no
// new connections are accepted, waits for in-flight requests to
// finish, and returns nil once the server is idle. If ctx's deadline
// expires first, remaining connections are closed and ctx's error is
// returned.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Routes builds the service's route table:
//
//	GET /         service index
//	GET /healthz  liveness probe
//	GET /readyz   readiness probe
//	GET /v1/events  the day's sun events for a place and date
//	GET /v1/moon  Moon illumination and phase for a UTC date
func Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /healthz", handleProbe("ok"))
	mux.HandleFunc("GET /readyz", handleProbe("ready"))
	mux.HandleFunc("GET /v1/events", handleEvents)
	mux.HandleFunc("GET /v1/moon", handleMoon)
	return mux
}

var endpoints = []string{"/", "/healthz", "/readyz", "/v1/events", "/v1/moon"}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":   "sol-server",
		"endpoints": endpoints,
	})
}

func handleProbe(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": status})
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status code is already sent; nothing more we can do.
		_ = err
	}
}
