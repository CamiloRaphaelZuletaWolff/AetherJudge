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

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.Env != "dev" {
		t.Errorf("Env = %q, want dev", cfg.Env)
	}
	if cfg.RedisAddr != "localhost:56379" {
		t.Errorf("RedisAddr = %q, want localhost:56379", cfg.RedisAddr)
	}
	if cfg.ExecutorAddr != "localhost:9090" {
		t.Errorf("ExecutorAddr = %q, want localhost:9090", cfg.ExecutorAddr)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 15m", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("RefreshTokenTTL = %v, want 720h", cfg.RefreshTokenTTL)
	}
	if cfg.JudgeWorkers != 2 {
		t.Errorf("JudgeWorkers = %d, want 2", cfg.JudgeWorkers)
	}
	if cfg.FrontendOrigin != "http://localhost:3000" {
		t.Errorf("FrontendOrigin = %q", cfg.FrontendOrigin)
	}
	if cfg.IsProduction() {
		t.Error("IsProduction() = true for dev defaults")
	}
	if cfg.ExecutorAuthToken != "" || cfg.ExecutorTLS {
		t.Errorf("executor security defaults = (token set: %v, tls: %v), want (false, false)",
			cfg.ExecutorAuthToken != "", cfg.ExecutorTLS)
	}
}

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("GATEWAY_HTTP_ADDR", ":9999")
	t.Setenv("DATABASE_URL", "postgres://db.internal:5432/arena_alt")
	t.Setenv("JUDGE_WORKERS", "8")
	t.Setenv("ACCESS_TOKEN_TTL_MIN", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr = %q, want :9999", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://db.internal:5432/arena_alt" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.JudgeWorkers != 8 {
		t.Errorf("JudgeWorkers = %d, want 8", cfg.JudgeWorkers)
	}
	if cfg.AccessTokenTTL != 5*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 5m", cfg.AccessTokenTTL)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"empty http addr", "GATEWAY_HTTP_ADDR", ""},
		{"unknown env", "APP_ENV", "staging"},
		{"empty database url", "DATABASE_URL", ""},
		{"non-integer workers", "JUDGE_WORKERS", "many"},
		{"negative workers", "JUDGE_WORKERS", "-1"},
		{"zero queue depth limit", "JUDGE_QUEUE_DEPTH_LIMIT", "0"},
		{"negative rate limit", "RATE_LIMIT_AUTH_PER_MIN", "-1"},
		{"non-boolean executor tls", "EXECUTOR_TLS", "yes please"},
		{"executor token with whitespace", "EXECUTOR_AUTH_TOKEN", "abc def"},
		{"executor token with trailing newline", "EXECUTOR_AUTH_TOKEN", "abcdef\n"},
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

func TestProductionRequiresRealJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	if _, err := Load(); err == nil {
		t.Fatal("production with dev JWT secret returned nil error, want error")
	}

	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("production with short JWT secret returned nil error, want error")
	}

	t.Setenv("JWT_SECRET", "replace-me-with-48-random-bytes-base64")
	if _, err := Load(); err == nil {
		t.Fatal("production with the .env.example placeholder returned nil error, want error")
	}

	t.Setenv("JWT_SECRET", "a-genuinely-long-random-secret-value-0123456789")
	if _, err := Load(); err != nil {
		t.Fatalf("production with strong secret: %v", err)
	}
}

func TestLoadExecutorSecurity(t *testing.T) {
	t.Setenv("EXECUTOR_ADDR", "203-0-113-7.sslip.io:443")
	t.Setenv("EXECUTOR_AUTH_TOKEN", "tok-0123456789")
	t.Setenv("EXECUTOR_TLS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ExecutorAddr != "203-0-113-7.sslip.io:443" {
		t.Errorf("ExecutorAddr = %q", cfg.ExecutorAddr)
	}
	if cfg.ExecutorAuthToken != "tok-0123456789" {
		t.Error("ExecutorAuthToken was not loaded from EXECUTOR_AUTH_TOKEN")
	}
	if !cfg.ExecutorTLS {
		t.Error("ExecutorTLS = false, want true")
	}

	// An explicitly empty EXECUTOR_TLS (e.g. a blank dashboard field) keeps
	// the plaintext default instead of failing startup.
	t.Setenv("EXECUTOR_TLS", "")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load with empty EXECUTOR_TLS: %v", err)
	}
	if cfg.ExecutorTLS {
		t.Error("empty EXECUTOR_TLS should mean false")
	}
}
