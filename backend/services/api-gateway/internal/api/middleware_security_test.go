package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caezu/arena/backend/services/api-gateway/internal/config"
)

func TestWithSecurityHeaders(t *testing.T) {
	t.Parallel()

	const csp = "default-src 'none'; frame-ancestors 'none'"

	tests := []struct {
		name      string
		env       string
		wantHSTS  bool
		wantValue map[string]string
	}{
		{
			name:     "dev omits HSTS",
			env:      "dev",
			wantHSTS: false,
			wantValue: map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Referrer-Policy":         "no-referrer",
				"Content-Security-Policy": csp,
			},
		},
		{
			name:     "production adds HSTS",
			env:      "production",
			wantHSTS: true,
			wantValue: map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Referrer-Policy":         "no-referrer",
				"Content-Security-Policy": csp,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &server{cfg: config.Config{Env: tt.env}}
			called := false
			h := s.withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/contests", nil))

			if !called {
				t.Fatal("next handler was not invoked")
			}
			for k, want := range tt.wantValue {
				if got := rec.Header().Get(k); got != want {
					t.Errorf("header %s = %q, want %q", k, got, want)
				}
			}
			hsts := rec.Header().Get("Strict-Transport-Security")
			if tt.wantHSTS && hsts == "" {
				t.Error("expected Strict-Transport-Security in production, got none")
			}
			if !tt.wantHSTS && hsts != "" {
				t.Errorf("expected no HSTS in dev, got %q", hsts)
			}
		})
	}
}
