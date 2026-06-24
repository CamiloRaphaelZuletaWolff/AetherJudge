package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Label values come from closed sets only (mux patterns, methods, status
// codes) — never raw paths or IDs — so series cardinality stays bounded.
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "arena_http_requests_total",
		Help: "HTTP requests served, by mux route pattern, method and status code.",
	}, []string{"route", "method", "code"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "arena_http_request_duration_seconds",
		Help:    "HTTP request latency by route and method (WebSocket upgrades excluded).",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method"})

	httpInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "arena_http_requests_in_flight",
		Help: "HTTP requests currently being served (WebSocket connections excluded).",
	})
)

const tracerName = "github.com/caezu/arena/backend/services/api-gateway/internal/httpserver"

// withTelemetry records one server span and the RED metrics per request.
//
// It must sit directly above the router: the span name and route label use
// req.Pattern, which the mux sets on the request it receives — any clone in
// between would hide it. WebSocket upgrades are counted but get neither a
// span nor a duration observation (they live for hours and would distort
// both; live connections are tracked by arena_ws_active_connections).
func withTelemetry(next http.Handler) http.Handler {
	tracer := otel.Tracer(tracerName)
	propagator := otel.GetTextMapPropagator()

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if isWebSocketUpgrade(req) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, req)
			httpRequestsTotal.WithLabelValues(routeLabel(req), req.Method, strconv.Itoa(rec.status)).Inc()
			return
		}

		ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))
		ctx, span := tracer.Start(ctx, req.Method, trace.WithSpanKind(trace.SpanKindServer))
		req = req.WithContext(ctx)

		httpInFlight.Inc()
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		finish := func() {
			httpInFlight.Dec()
			route := routeLabel(req)
			httpRequestsTotal.WithLabelValues(route, req.Method, strconv.Itoa(rec.status)).Inc()
			httpRequestDuration.WithLabelValues(route, req.Method).Observe(time.Since(start).Seconds())

			// The matched pattern is only known after routing, so the span
			// is named (and attributed) at the end of the request. Mux
			// patterns already carry the method ("GET /path").
			name := route
			if req.Pattern == "" {
				name = req.Method + " " + route
			}
			span.SetName(name)
			span.SetAttributes(
				attribute.String("http.request.method", req.Method),
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", rec.status),
				attribute.String("url.path", req.URL.Path),
			)
			if rec.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(rec.status))
			}
			span.End()
		}

		defer func() {
			if p := recover(); p != nil {
				// Account the request as a 500, then let the recovery
				// middleware above produce the actual response.
				rec.status = http.StatusInternalServerError
				finish()
				panic(p)
			}
			finish()
		}()

		next.ServeHTTP(rec, req)
	})
}

// routeLabel returns the matched mux pattern; unmatched requests (404/405)
// collapse into a single label so arbitrary paths cannot mint new series.
func routeLabel(req *http.Request) string {
	if req.Pattern != "" {
		return req.Pattern
	}
	return "unmatched"
}

func isWebSocketUpgrade(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}
