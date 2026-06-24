// Command worker runs Arena judge consumers: it pulls submissions from the
// durable Redis Streams queue, drives the executor, persists verdicts, and
// publishes live events — the same judging code the gateway can run in-process
// for local dev, here scaled as its own deployment (ADR-0011). It serves no
// public HTTP; only an internal /metrics listener.
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

	"github.com/caezu/arena/backend/services/api-gateway/internal/config"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/httpserver"
	"github.com/caezu/arena/backend/services/api-gateway/internal/judge"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "judge-worker:", err)
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
	if cfg.JudgeWorkers <= 0 {
		log.Warn("JUDGE_WORKERS is 0; this worker will consume nothing")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Init(ctx, log, "judge-worker", "dev")
	if err != nil {
		return fmt.Errorf("init telemetry: %w", err)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			log.Warn("shutdown tracing", "error", err)
		}
	}()

	// Migrations are session-locked (goose), so running them here is safe
	// regardless of web/worker start order and makes the worker self-sufficient.
	if err := db.Migrate(ctx, cfg.DatabaseURL, log); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	db.RegisterPoolMetrics(pool)
	store := db.New(pool)

	redis, err := redisx.Connect(ctx, cfg.RedisAddr, log)
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer func() {
		if err := redis.Close(); err != nil {
			log.Warn("close redis client", "error", err)
		}
	}()

	executorClient, executorConn, err := judge.Dial(cfg.ExecutorAddr)
	if err != nil {
		return fmt.Errorf("dial executor: %w", err)
	}
	defer func() {
		if err := executorConn.Close(); err != nil {
			log.Warn("close executor connection", "error", err)
		}
	}()

	judgeSvc := judge.New(store, executorClient, redis, redis, redis, judge.Config{
		Workers:         cfg.JudgeWorkers,
		ConsumerName:    cfg.ConsumerName,
		QueueDepthLimit: int64(cfg.JudgeQueueDepthLimit),
		MaxDeliveries:   int64(cfg.JudgeMaxDeliveries),
	}, log)
	if err := judgeSvc.StartConsumers(ctx); err != nil {
		return fmt.Errorf("start judge consumers: %w", err)
	}
	judgeSvc.StartQueueDepthSampler(ctx)

	log.Info("starting judge-worker",
		"metrics_addr", cfg.MetricsAddr,
		"judge_workers", cfg.JudgeWorkers,
		"consumer", cfg.ConsumerName,
		"executor_addr", cfg.ExecutorAddr,
	)

	// The only listener is internal /metrics; judging happens on the consumer
	// goroutines. A metrics-listener failure brings the process down so it
	// never runs unobserved.
	if err := httpserver.NewMetrics(cfg.MetricsAddr, log).Run(ctx); err != nil {
		return fmt.Errorf("run metrics server: %w", err)
	}

	judgeSvc.Wait()
	log.Info("judge-worker stopped cleanly")
	return nil
}
