package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
)

// rateLimitRejected counts 429s by scope ("auth", "submit", "run") — the
// abuse-pressure signal. Fail-closed 503s are visible in the HTTP metrics.
var rateLimitRejected = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "arena_rate_limit_rejected_total",
	Help: "Requests rejected by the fixed-window rate limiter, by scope.",
}, []string{"scope"})

type ctxKey int

const userCtxKey ctxKey = iota

// UserInfo identifies the authenticated caller. Role is the RBAC role carried
// in the access token (see ADR-0014); authorization checks read it via
// requirePermission.
type UserInfo struct {
	ID       uuid.UUID
	Username string
	Role     auth.Role
}

// UserFrom extracts the authenticated user placed by requireAuth.
func UserFrom(ctx context.Context) (UserInfo, bool) {
	u, ok := ctx.Value(userCtxKey).(UserInfo)
	return u, ok
}

// withCORS allows the single configured frontend origin, with credentials
// (the refresh cookie rides cross-origin requests in local dev).
func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == s.cfg.FrontendOrigin {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")

			if r.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders sets defensive response headers on every request. The
// gateway serves only JSON and WebSocket upgrades — never HTML — so a deny-all
// Content-Security-Policy is both safe and the tightest possible: there is no
// legitimate script, style, image, or frame content to allow. The X-Frame /
// nosniff / Referrer headers backstop older clients that ignore CSP.
func (s *server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		// HSTS only in production: it is meaningful solely over the HTTPS that
		// production terminates in front of the gateway, and would wrongly pin
		// a local http:// dev box for two years otherwise.
		if s.cfg.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth validates the access token (Authorization: Bearer header, or
// the access_token query parameter — the latter exists because browsers
// cannot set headers on WebSocket upgrades) and stores the user in context.
func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		} else if q := r.URL.Query().Get("access_token"); q != "" {
			token = q
		}
		if token == "" {
			respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
			return
		}

		claims, err := s.tokens.Verify(token)
		if err != nil {
			respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
			return
		}
		userID, err := claims.UserID()
		if err != nil {
			respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, UserInfo{
			ID:       userID,
			Username: claims.Username,
			Role:     auth.ParseRole(claims.Role),
		})
		next(w, r.WithContext(ctx))
	}
}

// requirePermission gates a handler on an RBAC permission. It must wrap
// requireAuth (which places the user, with role, in context). This is the real
// authorization boundary — the frontend's gating is only a UX convenience.
func (s *server) requirePermission(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFrom(r.Context())
		if !ok {
			respondError(w, s.log, http.StatusUnauthorized, "unauthorized", "missing access token")
			return
		}
		if !auth.Can(user.Role, perm) {
			respondError(w, s.log, http.StatusForbidden, "forbidden", "you do not have permission to do that")
			return
		}
		next(w, r)
	}
}

// rateLimited enforces a fixed-window limit per key. Fail-closed: if the
// limiter backend is down, abuse-sensitive routes refuse rather than open up.
func (s *server) rateLimited(scope string, perMin int, keyFn func(*http.Request) string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := scope + ":" + keyFn(r)
		allowed, err := s.redis.Allow(r.Context(), key, perMin, time.Minute)
		if err != nil {
			s.log.Error("rate limiter unavailable", "scope", scope, "error", err)
			respondError(w, s.log, http.StatusServiceUnavailable, "rate_limiter_unavailable", "try again shortly")
			return
		}
		if !allowed {
			rateLimitRejected.WithLabelValues(scope).Inc()
			respondError(w, s.log, http.StatusTooManyRequests, "rate_limited", "too many requests; slow down")
			return
		}
		next(w, r)
	}
}

// byIP keys rate limits on the client IP (no proxy in front during MVP;
// X-Forwarded-For handling arrives with the ingress in Phase 5).
func byIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// byUser keys rate limits on the authenticated user; falls back to IP.
func byUser(r *http.Request) string {
	if u, ok := UserFrom(r.Context()); ok {
		return u.ID.String()
	}
	return byIP(r)
}
