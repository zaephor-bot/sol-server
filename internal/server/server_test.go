package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		wantCode int
		wantBody map[string]string
	}{
		{
			name:     "index",
			method:   http.MethodGet,
			path:     "/",
			wantCode: http.StatusOK,
			wantBody: map[string]string{"service": "sol-server"},
		},
		{
			name:     "healthz",
			method:   http.MethodGet,
			path:     "/healthz",
			wantCode: http.StatusOK,
			wantBody: map[string]string{"status": "ok"},
		},
		{
			name:     "readyz",
			method:   http.MethodGet,
			path:     "/readyz",
			wantCode: http.StatusOK,
			wantBody: map[string]string{"status": "ready"},
		},
		{
			name:     "unknown path is 404",
			method:   http.MethodGet,
			path:     "/nope",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "non-GET is 405",
			method:   http.MethodPost,
			path:     "/healthz",
			wantCode: http.StatusMethodNotAllowed,
		},
	}

	mux := Routes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("%s %s = %d, want %d (body: %s)",
					tt.method, tt.path, rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantBody == nil {
				return
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("response is not a JSON object: %v (body: %s)", err, rec.Body.String())
			}
			for key, want := range tt.wantBody {
				if fmt.Sprint(got[key]) != want {
					t.Fatalf("%s %s: %q = %v, want %q", tt.method, tt.path, key, got[key], want)
				}
			}
		})
	}
}

func TestNewAddr(t *testing.T) {
	if got := New(9090).Addr(); got != ":9090" {
		t.Fatalf("New(9090).Addr() = %q, want \":9090\"", got)
	}
}

func TestShutdownDrainsInFlightRequests(t *testing.T) {
	port := freePort(t)
	srv := New(port)
	// Register a slow route so an in-flight request spans the shutdown.
	srv.srv.Handler.(*http.ServeMux).HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(750 * time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]string{"status": "done"})
	})

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	// A dedicated transport keeps this test's connections out of
	// http.DefaultTransport's process-wide pool, and DisableKeepAlives
	// makes every request use exactly one fresh connection, so no idle
	// pooled connection can delay Shutdown's quiescence check.
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	waitUntilListening(t, client, port)

	// Start a request that is still in flight when shutdown begins.
	inFlight := make(chan string, 1)
	go func() {
		resp, err := client.Get("http://" + listenAddr(port) + "/slow")
		if err != nil {
			inFlight <- err.Error()
			return
		}
		defer resp.Body.Close()
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			inFlight <- err.Error()
			return
		}
		inFlight <- body["status"]
	}()

	// Make sure the slow request has landed before shutting down.
	time.Sleep(150 * time.Millisecond)

	shutdownErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr <- srv.Shutdown(ctx)
	}()

	// The in-flight request must complete normally.
	if got := <-inFlight; got != "done" {
		t.Fatalf("in-flight request result = %q, want \"done\"", got)
	}

	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return after in-flight requests finished")
	}

	// ListenAndServe must report a clean stop, and new connections must
	// be refused now that the listener is closed.
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("ListenAndServe returned %v, want http.ErrServerClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe did not return after Shutdown")
	}
	if _, err := client.Get("http://" + listenAddr(port) + "/healthz"); err == nil {
		t.Fatal("new request after Shutdown unexpectedly succeeded")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func listenAddr(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func waitUntilListening(t *testing.T, client *http.Client, port int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + listenAddr(port) + "/healthz")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("server never became reachable")
}
