package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.GRPCAddr != ":9090" {
		t.Errorf("GRPCAddr = %q, want %q", cfg.GRPCAddr, ":9090")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.DockerHost != "" {
		t.Errorf("DockerHost = %q, want empty", cfg.DockerHost)
	}
	if cfg.ImageTag != "latest" {
		t.Errorf("ImageTag = %q, want %q", cfg.ImageTag, "latest")
	}
	if cfg.MaxConcurrent != 2 {
		t.Errorf("MaxConcurrent = %d, want 2", cfg.MaxConcurrent)
	}
	if cfg.RunTimeout != 2*time.Second {
		t.Errorf("RunTimeout = %v, want 2s", cfg.RunTimeout)
	}
	if cfg.RunTimeoutMax != 10*time.Second {
		t.Errorf("RunTimeoutMax = %v, want 10s", cfg.RunTimeoutMax)
	}
	if cfg.RunMemoryMB != 128 {
		t.Errorf("RunMemoryMB = %d, want 128", cfg.RunMemoryMB)
	}
	if cfg.RunMemoryMaxMB != 512 {
		t.Errorf("RunMemoryMaxMB = %d, want 512", cfg.RunMemoryMaxMB)
	}
	if cfg.CompileTimeout != 20*time.Second {
		t.Errorf("CompileTimeout = %v, want 20s", cfg.CompileTimeout)
	}
	if cfg.CompileMemoryMB != 512 {
		t.Errorf("CompileMemoryMB = %d, want 512", cfg.CompileMemoryMB)
	}
	if cfg.OutputLimitKB != 1024 {
		t.Errorf("OutputLimitKB = %d, want 1024", cfg.OutputLimitKB)
	}
}

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("EXECUTOR_GRPC_ADDR", "127.0.0.1:7777")
	t.Setenv("EXECUTOR_MAX_CONCURRENT", "8")
	t.Setenv("EXECUTOR_RUN_TIMEOUT_MS", "1500")
	t.Setenv("EXECUTOR_RUN_MEMORY_MB", "64")
	t.Setenv("EXECUTOR_IMAGE_TAG", "v2")
	t.Setenv("EXECUTOR_DOCKER_HOST", "tcp://10.0.0.5:2375")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.GRPCAddr != "127.0.0.1:7777" {
		t.Errorf("GRPCAddr = %q, want %q", cfg.GRPCAddr, "127.0.0.1:7777")
	}
	if cfg.MaxConcurrent != 8 {
		t.Errorf("MaxConcurrent = %d, want 8", cfg.MaxConcurrent)
	}
	if cfg.RunTimeout != 1500*time.Millisecond {
		t.Errorf("RunTimeout = %v, want 1.5s", cfg.RunTimeout)
	}
	if cfg.RunMemoryMB != 64 {
		t.Errorf("RunMemoryMB = %d, want 64", cfg.RunMemoryMB)
	}
	if cfg.ImageTag != "v2" {
		t.Errorf("ImageTag = %q, want %q", cfg.ImageTag, "v2")
	}
	if cfg.DockerHost != "tcp://10.0.0.5:2375" {
		t.Errorf("DockerHost = %q, want %q", cfg.DockerHost, "tcp://10.0.0.5:2375")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"empty grpc addr", "EXECUTOR_GRPC_ADDR", ""},
		{"non-integer concurrency", "EXECUTOR_MAX_CONCURRENT", "many"},
		{"zero concurrency", "EXECUTOR_MAX_CONCURRENT", "0"},
		{"negative timeout", "EXECUTOR_RUN_TIMEOUT_MS", "-5"},
		{"zero memory", "EXECUTOR_RUN_MEMORY_MB", "0"},
		{"max below default timeout", "EXECUTOR_RUN_TIMEOUT_MAX_MS", "100"},
		{"max below default memory", "EXECUTOR_RUN_MEMORY_MAX_MB", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)

			if _, err := Load(); err == nil {
				t.Errorf("Load with %s=%q returned nil error, want error", tt.key, tt.value)
			}
		})
	}
}
