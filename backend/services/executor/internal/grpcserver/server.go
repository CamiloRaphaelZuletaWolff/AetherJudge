// Package grpcserver hosts the executor's gRPC endpoint: the ExecutorService
// implementation plus the standard gRPC health protocol, with logging and
// panic-recovery interceptors.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

// Server is the executor gRPC server.
type Server struct {
	grpcSrv   *grpc.Server
	healthSrv *health.Server
	addr      string
	log       *slog.Logger
}

// New assembles a Server serving the ExecutorService implementation and the
// gRPC health protocol (the empty service name covers the whole process,
// which is what generic health probes target).
func New(addr string, log *slog.Logger, executor executorv1.ExecutorServiceServer) *Server {
	grpcSrv := grpc.NewServer(
		// Server spans continuing the gateway's trace context (no-op when
		// tracing is disabled).
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			recoveryInterceptor(log),
			loggingInterceptor(log),
		),
	)

	executorv1.RegisterExecutorServiceServer(grpcSrv, executor)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	return &Server{
		grpcSrv:   grpcSrv,
		healthSrv: healthSrv,
		addr:      addr,
		log:       log,
	}
}

// Run listens on the configured address and serves gRPC until ctx is
// cancelled, then performs a graceful stop.
func (s *Server) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpcserver: listen %s: %w", s.addr, err)
	}
	return s.Serve(ctx, lis)
}

// Serve runs the gRPC server on lis until ctx is cancelled. It is split from
// Run so tests can inject an in-memory listener.
func (s *Server) Serve(ctx context.Context, lis net.Listener) error {
	serveErr := make(chan error, 1)

	go func() {
		s.log.Info("grpc server listening", "addr", lis.Addr().String())
		// ErrServerStopped means GracefulStop won the race against this
		// goroutine starting — a clean shutdown, not a failure.
		if err := s.grpcSrv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serveErr <- fmt.Errorf("grpcserver: serve: %w", err)
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	s.log.Info("grpc server shutting down")
	s.healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	s.grpcSrv.GracefulStop()

	return <-serveErr
}
