// Command server runs the Arena api-gateway: the single public entrypoint
// terminating REST and WebSocket traffic, judging submissions through the
// executor, and fanning live contest events out over Redis Pub/Sub.
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

	"github.com/caezu/arena/backend/services/api-gateway/internal/api"
	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
	"github.com/caezu/arena/backend/services/api-gateway/internal/config"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
	"github.com/caezu/arena/backend/services/api-gateway/internal/httpserver"
	"github.com/caezu/arena/backend/services/api-gateway/internal/judge"
	"github.com/caezu/arena/backend/services/api-gateway/internal/realtime"
	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api-gateway:", err)
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
	if cfg.UsingWeakJWTSecret() {
		log.Warn("running with a known development JWT secret; run 'task env:init' to generate one")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Init(ctx, log, "api-gateway", "dev")
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

	// redis satisfies the queue, leaderboard-cache, and broadcaster surfaces.
	judgeSvc := judge.New(store, executorClient, redis, redis, redis, judge.Config{
		Workers:         cfg.JudgeWorkers,
		ConsumerName:    cfg.ConsumerName,
		QueueDepthLimit: int64(cfg.JudgeQueueDepthLimit),
		MaxDeliveries:   int64(cfg.JudgeMaxDeliveries),
	}, log)
	// The gateway both produces (HTTP submit) and, when JUDGE_WORKERS>0, runs
	// consumers — convenient for single-process local dev. In Kubernetes the
	// web tier sets JUDGE_WORKERS=0 and a separate worker Deployment judges.
	if err := judgeSvc.StartConsumers(ctx); err != nil {
		return fmt.Errorf("start judge consumers: %w", err)
	}
	judgeSvc.StartQueueDepthSampler(ctx)

	router := api.NewRouter(api.Deps{
		Cfg:      cfg,
		Log:      log,
		Store:    store,
		Redis:    redis,
		Tokens:   auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL),
		Refresh:  auth.NewRefreshManager(store, cfg.RefreshTokenTTL, log),
		Judge:    judgeSvc,
		Hub:      realtime.NewHub(ctx, redis, log),
		Executor: executorClient,
	})

	log.Info("starting api-gateway",
		"http_addr", cfg.HTTPAddr,
		"metrics_addr", cfg.MetricsAddr,
		"judge_workers", cfg.JudgeWorkers,
		"executor_addr", cfg.ExecutorAddr,
	)

	// Public API and internal /metrics run as separate listeners; a failure
	// of either (e.g. a busy port at startup) brings both down so the
	// process fails loudly instead of running half-instrumented.
	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	srvErrs := make(chan error, 2)
	go func() { srvErrs <- httpserver.New(cfg.HTTPAddr, log, router).Run(srvCtx) }()
	go func() { srvErrs <- httpserver.NewMetrics(cfg.MetricsAddr, log).Run(srvCtx) }()

	var firstErr error
	for range 2 {
		if err := <-srvErrs; err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}
	if firstErr != nil {
		return fmt.Errorf("run http servers: %w", firstErr)
	}

	judgeSvc.Wait()
	log.Info("api-gateway stopped cleanly")
	return nil
}
