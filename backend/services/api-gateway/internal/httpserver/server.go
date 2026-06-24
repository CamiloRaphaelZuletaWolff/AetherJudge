// Package httpserver provides the gateway's HTTP chassis: shared middleware
// (request logging, panic recovery) and graceful lifecycle management around
// a router built elsewhere (internal/api).
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// shutdownTimeout bounds how long in-flight requests may take to drain once
// the process receives a termination signal.
const shutdownTimeout = 10 * time.Second

// Server is the api-gateway HTTP server.
type Server struct {
	httpSrv *http.Server
	log     *slog.Logger
}

// New wraps handler with the chassis middleware and returns a Server ready
// to Run.
//
// Order matters: recovery outermost (catches everything, including the other
// middleware), then telemetry (so the access log inside it can carry the
// span's trace_id), then request logging, then the router.
func New(addr string, log *slog.Logger, handler http.Handler) *Server {
	wrapped := withRecovery(log, withTelemetry(withRequestLogging(log, handler)))

	return &Server{
		httpSrv: &http.Server{
			Addr:              addr,
			Handler:           wrapped,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			// WriteTimeout would sever long-lived WebSocket connections;
			// per-write deadlines are enforced in internal/realtime instead.
			IdleTimeout: 60 * time.Second,
		},
		log: log,
	}
}

// Handler exposes the fully assembled middleware chain so tests can exercise
// it without binding a real port.
func (s *Server) Handler() http.Handler { return s.httpSrv.Handler }

// Run serves HTTP until ctx is cancelled, then drains in-flight requests for
// up to shutdownTimeout. It blocks and returns nil after a clean shutdown.
func (s *Server) Run(ctx context.Context) error {
	serveErr := make(chan error, 1)

	go func() {
		s.log.Info("http server listening", "addr", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("httpserver: serve: %w", err)
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	s.log.Info("http server shutting down", "timeout", shutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}

	return <-serveErr
}
