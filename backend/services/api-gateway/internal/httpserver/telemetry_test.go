package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// installTestTracer routes the global tracer provider into an in-memory
// exporter. Tests using it must not run in parallel (global state).
func installTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)))
	t.Cleanup(func() { otel.SetTracerProvider(old) })
	return exporter
}

func TestTelemetryRecordsRoutePatternAndSpan(t *testing.T) {
	exporter := installTestTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/contests/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := withTelemetry(mux)

	const route = "GET /api/v1/contests/{id}"
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(route, http.MethodGet, "200"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contests/1234", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(route, http.MethodGet, "200"))
	if after-before != 1 {
		t.Errorf("requests_total{route=%q} delta = %v, want 1 (route label must be the mux pattern, not the raw path)", route, after-before)
	}

	// Filter by name: parallel lifecycle tests in this package also serve
	// requests through the chain and may emit "GET unmatched" spans here.
	var span *tracetest.SpanStub
	for i, s := range exporter.GetSpans() {
		if s.Name == route {
			span = &exporter.GetSpans()[i]
			break
		}
	}
	if span == nil {
		t.Fatalf("no span named %q exported (span must be renamed to the matched pattern after routing)", route)
	}
	if span.SpanKind != oteltrace.SpanKindServer {
		t.Errorf("span kind = %v, want server", span.SpanKind)
	}
}

func TestTelemetryUnmatchedRouteCollapses(t *testing.T) {
	installTestTracer(t)

	handler := withTelemetry(http.NewServeMux())

	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("unmatched", http.MethodGet, "404"))

	req := httptest.NewRequest(http.MethodGet, "/this/path/should/not/mint/a/series", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("unmatched", http.MethodGet, "404"))
	if after-before != 1 {
		t.Errorf(`requests_total{route="unmatched"} delta = %v, want 1`, after-before)
	}
}

func TestTelemetrySkipsWebSocketUpgrades(t *testing.T) {
	exporter := installTestTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ws/contests/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := withTelemetry(mux)

	const route = "GET /api/v1/ws/contests/{id}"
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(route, http.MethodGet, "200"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/contests/1234", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(route, http.MethodGet, "200"))
	if after-before != 1 {
		t.Errorf("upgrade request not counted: delta = %v, want 1", after-before)
	}
	for _, s := range exporter.GetSpans() {
		if s.Name == route {
			t.Errorf("upgrade request produced span %q, want none (connection-lifetime spans are useless)", s.Name)
		}
	}
}

func TestTelemetryAccountsPanicsAsServerErrors(t *testing.T) {
	installTestTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	})
	handler := withTelemetry(mux)

	const route = "GET /boom"
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(route, http.MethodGet, "500"))

	func() {
		defer func() {
			if recover() == nil {
				t.Error("withTelemetry swallowed the panic; the recovery middleware above it must see it")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	}()

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(route, http.MethodGet, "500"))
	if after-before != 1 {
		t.Errorf("panicking request delta = %v, want 1 counted as 500", after-before)
	}
}
