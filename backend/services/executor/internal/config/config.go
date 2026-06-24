// Package config loads executor settings from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting for the executor process.
type Config struct {
	// GRPCAddr is the listen address for the internal gRPC server, e.g. ":9090".
	GRPCAddr string
	// MetricsAddr is the listen address for the internal /metrics server.
	MetricsAddr string
	// LogLevel is one of debug, info, warn, error (validated by pkg/logging).
	LogLevel string
	// LogFormat is "json" (production) or "text" (local development).
	LogFormat string

	// DockerHost overrides the Docker daemon endpoint. Empty means standard
	// environment resolution (named pipe on Windows, /var/run/docker.sock on
	// Linux, or DOCKER_HOST).
	DockerHost string
	// ImageTag selects the sandbox image tag (arena-sandbox-<lang>:<tag>).
	ImageTag string

	// MaxConcurrent bounds how many sandboxes run simultaneously.
	MaxConcurrent int

	// RunTimeout is the default wall-clock limit for the run phase; requests
	// may lower it or raise it up to RunTimeoutMax.
	RunTimeout    time.Duration
	RunTimeoutMax time.Duration
	// RunMemoryMB is the default run-phase memory limit; requests may lower
	// it or raise it up to RunMemoryMaxMB.
	RunMemoryMB    int64
	RunMemoryMaxMB int64

	// CompileTimeout and CompileMemoryMB bound the compile phase. These are
	// server policy only — never client-controlled.
	CompileTimeout  time.Duration
	CompileMemoryMB int64

	// OutputLimitKB caps how much stdout/stderr is captured per stream.
	OutputLimitKB int64
}

// Load reads configuration from environment variables, applying defaults
// suitable for local development.
func Load() (Config, error) {
	cfg := Config{
		GRPCAddr: getEnv("EXECUTOR_GRPC_ADDR", ":9090"),
		// 9101: one above the gateway's 9100, same internal-only rule.
		MetricsAddr: getEnv("EXECUTOR_METRICS_ADDR", ":9101"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		LogFormat:   getEnv("LOG_FORMAT", "json"),
		DockerHost:  getEnv("EXECUTOR_DOCKER_HOST", ""),
		ImageTag:    getEnv("EXECUTOR_IMAGE_TAG", "latest"),
	}

	if cfg.GRPCAddr == "" {
		return Config{}, errors.New("config: EXECUTOR_GRPC_ADDR must not be empty")
	}
	if cfg.MetricsAddr == "" {
		return Config{}, errors.New("config: EXECUTOR_METRICS_ADDR must not be empty")
	}
	if cfg.MetricsAddr == cfg.GRPCAddr {
		return Config{}, errors.New("config: EXECUTOR_METRICS_ADDR must differ from EXECUTOR_GRPC_ADDR")
	}
	if cfg.ImageTag == "" {
		return Config{}, errors.New("config: EXECUTOR_IMAGE_TAG must not be empty")
	}

	intFields := []struct {
		dst      *int64
		key      string
		fallback int64
	}{
		{&cfg.RunMemoryMB, "EXECUTOR_RUN_MEMORY_MB", 128},
		{&cfg.RunMemoryMaxMB, "EXECUTOR_RUN_MEMORY_MAX_MB", 512},
		{&cfg.CompileMemoryMB, "EXECUTOR_COMPILE_MEMORY_MB", 512},
		{&cfg.OutputLimitKB, "EXECUTOR_OUTPUT_LIMIT_KB", 1024},
	}
	for _, f := range intFields {
		v, err := getEnvInt(f.key, f.fallback)
		if err != nil {
			return Config{}, err
		}
		*f.dst = v
	}

	maxConcurrent, err := getEnvInt("EXECUTOR_MAX_CONCURRENT", 2)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxConcurrent = int(maxConcurrent)

	durationFields := []struct {
		dst      *time.Duration
		key      string
		fallback int64 // milliseconds
	}{
		{&cfg.RunTimeout, "EXECUTOR_RUN_TIMEOUT_MS", 2000},
		{&cfg.RunTimeoutMax, "EXECUTOR_RUN_TIMEOUT_MAX_MS", 10000},
		{&cfg.CompileTimeout, "EXECUTOR_COMPILE_TIMEOUT_MS", 20000},
	}
	for _, f := range durationFields {
		ms, err := getEnvInt(f.key, f.fallback)
		if err != nil {
			return Config{}, err
		}
		*f.dst = time.Duration(ms) * time.Millisecond
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	positive := []struct {
		name  string
		value int64
	}{
		{"EXECUTOR_MAX_CONCURRENT", int64(c.MaxConcurrent)},
		{"EXECUTOR_RUN_TIMEOUT_MS", int64(c.RunTimeout)},
		{"EXECUTOR_RUN_TIMEOUT_MAX_MS", int64(c.RunTimeoutMax)},
		{"EXECUTOR_RUN_MEMORY_MB", c.RunMemoryMB},
		{"EXECUTOR_RUN_MEMORY_MAX_MB", c.RunMemoryMaxMB},
		{"EXECUTOR_COMPILE_TIMEOUT_MS", int64(c.CompileTimeout)},
		{"EXECUTOR_COMPILE_MEMORY_MB", c.CompileMemoryMB},
		{"EXECUTOR_OUTPUT_LIMIT_KB", c.OutputLimitKB},
	}
	for _, p := range positive {
		if p.value <= 0 {
			return fmt.Errorf("config: %s must be positive", p.name)
		}
	}

	if c.RunTimeoutMax < c.RunTimeout {
		return errors.New("config: EXECUTOR_RUN_TIMEOUT_MAX_MS must be >= EXECUTOR_RUN_TIMEOUT_MS")
	}
	if c.RunMemoryMaxMB < c.RunMemoryMB {
		return errors.New("config: EXECUTOR_RUN_MEMORY_MAX_MB must be >= EXECUTOR_RUN_MEMORY_MB")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int64) (int64, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", key, v)
	}
	return n, nil
}
