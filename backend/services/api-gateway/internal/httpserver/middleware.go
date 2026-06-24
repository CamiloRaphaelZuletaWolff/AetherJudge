package httpserver

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// statusRecorder captures the response status code for request logging.
//
// It must keep the wrapped writer's optional interfaces reachable: WebSocket
// upgrades need http.Hijacker, and hiding it behind a plain embed makes
// every upgrade fail (the classic middleware pitfall — found by the Phase 4
// E2E suite when ws connections started answering 501).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack exposes the underlying connection for protocol upgrades.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("httpserver: underlying writer does not support hijacking")
	}
	conn, rw, err := h.Hijack()
	if err == nil {
		// The connection now speaks WebSocket; HTTP status semantics are
		// over. Record the switching-protocols code for the access log.
		r.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

// Flush passes streaming writes through.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap supports http.ResponseController-based access to the original
// writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// withRequestLogging emits one structured log line per completed request.
func withRequestLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, req)

		// InfoContext so the trace handler can attach trace_id/span_id from
		// the span opened by withTelemetry above this middleware.
		log.InfoContext(req.Context(), "http request",
			"method", req.Method,
			"path", req.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", req.RemoteAddr,
		)
	})
}

// withRecovery converts handler panics into 500 responses instead of letting
// a single bad request crash the whole process.
func withRecovery(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("handler panic",
					"panic", rec,
					"method", req.Method,
					"path", req.URL.Path,
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, req)
	})
}
