// Command sol-server runs the sol HTTP service.
//
// Configuration: the listen port is read from the PORT environment
// variable and falls back to 8080 when unset (or invalid; see
// internal/config).
//
// Shutdown: on SIGINT or SIGTERM the server stops accepting new
// connections, lets in-flight requests finish, and then exits with
// status 0. A bounded drain deadline keeps the exit ahead of an
// orchestrator's SIGKILL.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zaephor-bot/sol-server/internal/config"
	"github.com/zaephor-bot/sol-server/internal/server"
)

// shutdownTimeout bounds how long shutdown waits for in-flight
// requests to finish before abandoning them. It is deliberately
// shorter than common orchestrator grace periods (Kubernetes defaults
// to 30s) so the process still exits on its own terms.
const shutdownTimeout = 10 * time.Second

func main() {
	logger := log.New(os.Stderr, "sol-server: ", log.LstdFlags)
	if err := run(logger); err != nil {
		logger.Printf("fatal: %v", err)
		os.Exit(1)
	}
	logger.Print("shutdown complete")
}

func run(logger *log.Logger) error {
	port, err := config.Port(os.Getenv)
	if err != nil {
		logger.Printf("config: %v; falling back to port %d", err, config.DefaultPort)
		port = config.DefaultPort
	}

	srv := server.New(port)

	// Register the signal handler before the server starts so a
	// signal arriving during startup is still handled gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s", srv.Addr())
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Print("signal received; draining in-flight requests")
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
