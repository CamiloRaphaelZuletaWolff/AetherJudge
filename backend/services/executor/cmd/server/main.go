// Command server runs the Arena executor: the isolated service that compiles
// and runs untrusted user code inside sandboxed Docker containers and judges
// the results (see docs/adr/0005 and docs/adr/0006).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caezu/arena/backend/pkg/logging"
	"github.com/caezu/arena/backend/pkg/telemetry"

	"github.com/caezu/arena/backend/services/executor/internal/config"
	"github.com/caezu/arena/backend/services/executor/internal/grpcserver"
	"github.com/caezu/arena/backend/services/executor/internal/metricsserver"
	"github.com/caezu/arena/backend/services/executor/internal/sandbox"
	"github.com/caezu/arena/backend/services/executor/internal/service"
)

// dockerConnectTimeout bounds the startup ping to the Docker daemon.
const dockerConnectTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "executor:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logging.New(os.Stdout, cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Init(ctx, log, "executor", "dev")
	if err != nil {
		return fmt.Errorf("init telemetry: %w", err)
	}
	defer func() {
		// Fresh context: ctx is already cancelled during shutdown, and the
		// final span flush deserves a bounded grace period.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			log.Warn("shutdown tracing", "error", err)
		}
	}()

	connectCtx, cancel := context.WithTimeout(ctx, dockerConnectTimeout)
	defer cancel()

	engine, err := sandbox.NewEngine(connectCtx, cfg.DockerHost, log)
	if err != nil {
		return fmt.Errorf("connect to docker: %w", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			log.Warn("close sandbox engine", "error", err)
		}
	}()

	log.Info("starting executor",
		"grpc_addr", cfg.GRPCAddr,
		"metrics_addr", cfg.MetricsAddr,
		"max_concurrent", cfg.MaxConcurrent,
		"image_tag", cfg.ImageTag,
	)

	// gRPC and the internal /metrics listener run together; a failure of
	// either (e.g. a busy port at startup) brings both down so the process
	// fails loudly instead of running half-instrumented.
	srvCtx, cancelSrv := context.WithCancel(ctx)
	defer cancelSrv()

	svc := service.New(engine, cfg, log)
	srvErrs := make(chan error, 2)
	go func() { srvErrs <- grpcserver.New(cfg.GRPCAddr, log, svc).Run(srvCtx) }()
	go func() { srvErrs <- metricsserver.New(cfg.MetricsAddr, log).Run(srvCtx) }()

	var firstErr error
	for range 2 {
		if err := <-srvErrs; err != nil && firstErr == nil {
			firstErr = err
			cancelSrv()
		}
	}
	if firstErr != nil {
		return fmt.Errorf("run servers: %w", firstErr)
	}

	log.Info("executor stopped cleanly")
	return nil
}
