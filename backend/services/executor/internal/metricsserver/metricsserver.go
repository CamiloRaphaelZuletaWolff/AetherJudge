// Package metricsserver exposes the executor's Prometheus registry over a
// small dedicated HTTP listener (the service itself is gRPC-only, and the
// gRPC port must stay free of operational endpoints).
package metricsserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// shutdownTimeout bounds the drain after ctx cancellation; scrapes are
// sub-second, so this is generous.
const shutdownTimeout = 5 * time.Second

// Server serves GET /metrics.
type Server struct {
	httpSrv *http.Server
	log     *slog.Logger
}

// New builds the metrics server for addr.
func New(addr string, log *slog.Logger) *Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	return &Server{
		httpSrv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		log: log,
	}
}

// Run serves until ctx is cancelled, then shuts down gracefully. It blocks
// and returns nil after a clean shutdown.
func (s *Server) Run(ctx context.Context) error {
	serveErr := make(chan error, 1)

	go func() {
		s.log.Info("metrics server listening", "addr", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("metricsserver: serve: %w", err)
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("metricsserver: shutdown: %w", err)
	}

	return <-serveErr
}
