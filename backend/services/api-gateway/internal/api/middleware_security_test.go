package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

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

func TestClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hops int
		xff  []string
		want string
	}{
		{name: "no proxy trust ignores XFF", hops: 0, xff: []string{"203.0.113.9"}, want: "10.0.0.1"},
		{name: "one hop takes rightmost entry", hops: 1, xff: []string{"203.0.113.9"}, want: "203.0.113.9"},
		{name: "spoofed left entries are ignored", hops: 1, xff: []string{"1.2.3.4, 203.0.113.9"}, want: "203.0.113.9"},
		{name: "multiple headers form one chain", hops: 2, xff: []string{"1.2.3.4", "203.0.113.9, 198.51.100.7"}, want: "203.0.113.9"},
		{name: "short chain falls back to peer", hops: 2, xff: []string{"203.0.113.9"}, want: "10.0.0.1"},
		{name: "garbage falls back to peer", hops: 1, xff: []string{"not-an-ip"}, want: "10.0.0.1"},
		{name: "missing header falls back to peer", hops: 1, want: "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &server{cfg: config.Config{TrustedProxyHops: tt.hops}}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			r.RemoteAddr = "10.0.0.1:5555"
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := s.clientIP(r); got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConnLimiter(t *testing.T) {
	t.Parallel()

	l := newConnLimiter(2)
	a, b := uuid.New(), uuid.New()
	for i := range 2 {
		if !l.acquire(a) {
			t.Fatalf("acquire #%d must succeed under the cap", i+1)
		}
	}
	if l.acquire(a) {
		t.Error("third acquire must fail at the cap")
	}
	if !l.acquire(b) {
		t.Error("another user has an independent budget")
	}
	l.release(a)
	if !l.acquire(a) {
		t.Error("acquire after release must succeed")
	}
	l.release(a)
	l.release(a)
	l.release(b)
	if len(l.count) != 0 {
		t.Errorf("count map = %v, want empty after releasing everything", l.count)
	}
}
