// Package config loads api-gateway settings from the environment.
//
// Environment variables are the single configuration mechanism so the same
// binary runs unchanged under Docker Compose and Kubernetes.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// devJWTSecret is the development fallback. Production mode refuses to start
// with it — see validate.
//
//nolint:gosec // intentionally hardcoded dev-only value, rejected in production
const devJWTSecret = "dev-secret-do-not-use-in-production"

// Config holds every runtime setting for the api-gateway process.
type Config struct {
	// HTTPAddr is the listen address for the public HTTP server, e.g. ":8080".
	HTTPAddr string
	// MetricsAddr is the listen address for the internal /metrics server.
	MetricsAddr string
	// LogLevel is one of debug, info, warn, error (validated by pkg/logging).
	LogLevel string
	// LogFormat is "json" (production) or "text" (local development).
	LogFormat string
	// Env is "dev" or "production"; production tightens validation.
	Env string

	// DatabaseURL is the PostgreSQL DSN.
	DatabaseURL string
	// RedisAddr is the Redis host:port.
	RedisAddr string
	// ExecutorAddr is the executor gRPC host:port.
	ExecutorAddr string
	// ExecutorAuthToken is the shared secret sent to the executor as
	// `authorization: Bearer <token>` on every RPC. Empty sends nothing
	// (local dev / in-cluster executors with auth disabled).
	ExecutorAuthToken string
	// ExecutorTLS dials the executor over TLS (system root CAs) instead of
	// plaintext — required when it sits behind a public TLS proxy.
	ExecutorTLS bool

	// JWTSecret signs access tokens (HS256).
	JWTSecret string
	// AccessTokenTTL bounds access-token lifetime.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL bounds refresh-token lifetime.
	RefreshTokenTTL time.Duration

	// FrontendOrigin is the single allowed CORS origin (credentials mode).
	FrontendOrigin string

	// JudgeWorkers is the number of consumer goroutines this process runs
	// against the durable Redis judge queue (0 = producer-only web tier).
	JudgeWorkers int
	// JudgeQueueDepthLimit rejects new submissions with HTTP 503 once the
	// stream holds this many items (backpressure).
	JudgeQueueDepthLimit int
	// JudgeMaxDeliveries dead-letters a submission after this many failed
	// delivery attempts (poison-message backstop).
	JudgeMaxDeliveries int
	// ConsumerName identifies this process in the judge consumer group; in
	// Kubernetes it is set to the pod name.
	ConsumerName string

	// TrustedProxyHops is how many reverse proxies in front of the gateway
	// append to X-Forwarded-For (0 = none: rate limits key on the TCP peer).
	// On Render use 1: the entry its edge appends is never client-supplied.
	// Never set it higher than the real hop count: the extra entries are
	// client-controlled and would let an attacker pick their own rate-limit
	// key.
	TrustedProxyHops int

	// Rate limits, per principal per minute.
	AuthRatePerMin   int
	SubmitRatePerMin int
	RunRatePerMin    int
}

// Load reads configuration from environment variables, applying defaults
// suitable for local development.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr: getEnv("GATEWAY_HTTP_ADDR", ":8080"),
		// 9100 by convention (node_exporter's family); the public listener
		// must never serve /metrics.
		MetricsAddr: getEnv("GATEWAY_METRICS_ADDR", ":9100"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		LogFormat:   getEnv("LOG_FORMAT", "json"),
		Env:         getEnv("APP_ENV", "dev"),
		// Ports match infra/docker/docker-compose.yml's host mappings
		// (non-standard on purpose; see the comments there).
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://arena:arena_dev_password@localhost:55432/arena?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:56379"),
		ExecutorAddr:   getEnv("EXECUTOR_ADDR", "localhost:9090"),
		JWTSecret:      getEnv("JWT_SECRET", devJWTSecret),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:3000"),
		ConsumerName:   getEnv("CONSUMER_NAME", "gateway"),
		// Never defaulted: an empty token means "send no auth header".
		ExecutorAuthToken: getEnv("EXECUTOR_AUTH_TOKEN", ""),
	}

	intFields := []struct {
		dst      *int
		key      string
		fallback int
	}{
		{&cfg.JudgeWorkers, "JUDGE_WORKERS", 2},
		{&cfg.JudgeQueueDepthLimit, "JUDGE_QUEUE_DEPTH_LIMIT", 1024},
		{&cfg.JudgeMaxDeliveries, "JUDGE_MAX_DELIVERIES", 5},
		{&cfg.AuthRatePerMin, "RATE_LIMIT_AUTH_PER_MIN", 10},
		{&cfg.SubmitRatePerMin, "RATE_LIMIT_SUBMIT_PER_MIN", 6},
		{&cfg.RunRatePerMin, "RATE_LIMIT_RUN_PER_MIN", 10},
		{&cfg.TrustedProxyHops, "TRUSTED_PROXY_HOPS", 0},
	}
	for _, f := range intFields {
		v, err := getEnvInt(f.key, f.fallback)
		if err != nil {
			return Config{}, err
		}
		*f.dst = v
	}

	executorTLS, err := getEnvBool("EXECUTOR_TLS", false)
	if err != nil {
		return Config{}, err
	}
	cfg.ExecutorTLS = executorTLS

	accessMin, err := getEnvInt("ACCESS_TOKEN_TTL_MIN", 15)
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTokenTTL = time.Duration(accessMin) * time.Minute

	refreshHours, err := getEnvInt("REFRESH_TOKEN_TTL_HOURS", 720)
	if err != nil {
		return Config{}, err
	}
	cfg.RefreshTokenTTL = time.Duration(refreshHours) * time.Hour

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// IsProduction reports whether the gateway runs in production mode.
func (c Config) IsProduction() bool { return c.Env == "production" }

// maxTrustedProxyHops bounds TRUSTED_PROXY_HOPS; no sane deployment chains
// more reverse proxies than this.
const maxTrustedProxyHops = 5

// envPlaceholderSecret is the value shipped in .env.example; it must never
// be accepted in production either.
const envPlaceholderSecret = "replace-me-with-48-random-bytes-base64"

// UsingWeakJWTSecret reports whether the signing secret is a known
// development value (used for a startup warning in dev mode).
func (c Config) UsingWeakJWTSecret() bool {
	return c.JWTSecret == devJWTSecret || c.JWTSecret == envPlaceholderSecret
}

func (c Config) validate() error {
	if c.HTTPAddr == "" {
		return errors.New("config: GATEWAY_HTTP_ADDR must not be empty")
	}
	if c.MetricsAddr == "" {
		return errors.New("config: GATEWAY_METRICS_ADDR must not be empty")
	}
	if c.MetricsAddr == c.HTTPAddr {
		return errors.New("config: GATEWAY_METRICS_ADDR must differ from GATEWAY_HTTP_ADDR")
	}
	if c.Env != "dev" && c.Env != "production" {
		return fmt.Errorf("config: APP_ENV must be dev or production, got %q", c.Env)
	}
	for _, s := range []struct{ name, v string }{
		{"DATABASE_URL", c.DatabaseURL},
		{"REDIS_ADDR", c.RedisAddr},
		{"EXECUTOR_ADDR", c.ExecutorAddr},
		{"FRONTEND_ORIGIN", c.FrontendOrigin},
	} {
		if s.v == "" {
			return fmt.Errorf("config: %s must not be empty", s.name)
		}
	}

	// A pasted token with a stray space/newline would never match the
	// executor's and fail every judge call with a confusing Unauthenticated.
	if strings.ContainsAny(c.ExecutorAuthToken, " \t\r\n") {
		return errors.New("config: EXECUTOR_AUTH_TOKEN must not contain whitespace")
	}

	if c.IsProduction() && (c.UsingWeakJWTSecret() || len(c.JWTSecret) < 32) {
		return errors.New("config: production requires JWT_SECRET set to a strong value (>= 32 chars)")
	}

	positive := []struct {
		name  string
		value int64
	}{
		{"ACCESS_TOKEN_TTL_MIN", int64(c.AccessTokenTTL)},
		{"REFRESH_TOKEN_TTL_HOURS", int64(c.RefreshTokenTTL)},
		{"JUDGE_QUEUE_DEPTH_LIMIT", int64(c.JudgeQueueDepthLimit)},
		{"JUDGE_MAX_DELIVERIES", int64(c.JudgeMaxDeliveries)},
		{"RATE_LIMIT_AUTH_PER_MIN", int64(c.AuthRatePerMin)},
		{"RATE_LIMIT_SUBMIT_PER_MIN", int64(c.SubmitRatePerMin)},
		{"RATE_LIMIT_RUN_PER_MIN", int64(c.RunRatePerMin)},
	}
	for _, p := range positive {
		if p.value <= 0 {
			return fmt.Errorf("config: %s must be positive", p.name)
		}
	}
	if c.TrustedProxyHops < 0 || c.TrustedProxyHops > maxTrustedProxyHops {
		return fmt.Errorf("config: TRUSTED_PROXY_HOPS must be between 0 and %d", maxTrustedProxyHops)
	}
	// Workers may be 0 (a producer-only web tier) but never negative.
	if c.JudgeWorkers < 0 {
		return errors.New("config: JUDGE_WORKERS must not be negative")
	}
	if c.ConsumerName == "" {
		return errors.New("config: CONSUMER_NAME must not be empty")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s must be a boolean (true/false), got %q", key, v)
	}
	return b, nil
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", key, v)
	}
	return n, nil
}
