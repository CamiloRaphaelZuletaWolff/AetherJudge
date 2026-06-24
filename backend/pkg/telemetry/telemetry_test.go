package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestInitWithoutEndpointIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	shutdown, err := Init(context.Background(), log, "test-service", "v0")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Init returned nil shutdown function")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}

	// The global provider must not be an SDK provider when disabled.
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		t.Error("tracing disabled but a real SDK tracer provider was installed")
	}

	// Propagation must still be wired so trace context flows through.
	if fields := otel.GetTextMapPropagator().Fields(); len(fields) == 0 {
		t.Error("no propagator installed when tracing is disabled")
	}
}

func TestInitWithEndpointInstallsProvider(t *testing.T) {
	// The OTLP exporter dials lazily; no collector needs to listen here.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:14317")

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	shutdown, err := Init(context.Background(), log, "test-service", "v0")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		// Restore a no-op provider for other tests.
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Error("tracing enabled but global provider is not the SDK provider")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
