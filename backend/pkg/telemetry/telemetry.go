// Package telemetry configures OpenTelemetry tracing for Arena services.
//
// The contract is fail-soft: when OTEL_EXPORTER_OTLP_ENDPOINT is unset the
// global tracer provider stays a no-op and Init costs nothing — services
// must never degrade because telemetry is absent or broken. Export errors
// are logged, not propagated.
package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// envEndpoint is the standard OTel SDK variable naming the OTLP collector.
// Its presence is the on/off switch for tracing.
const envEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"

// Init installs the global tracer provider and W3C propagators.
//
// The returned shutdown function flushes pending spans and must be called
// before process exit (it is a no-op when tracing is disabled). The W3C
// propagator is installed even when tracing is off so trace context still
// flows through this service untouched.
func Init(ctx context.Context, log *slog.Logger, service, version string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if os.Getenv(envEndpoint) == "" {
		log.Info("tracing disabled", "reason", envEndpoint+" not set")
		return func(context.Context) error { return nil }, nil
	}

	// Failed or slow exports must never surface as request errors.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Warn("otel export error", "error", err)
	}))

	// The exporter reads endpoint/headers/TLS settings from the standard
	// OTEL_EXPORTER_OTLP_* variables; it dials lazily, so an unreachable
	// collector delays nothing at startup.
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("telemetry: create otlp exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(service),
		semconv.ServiceVersion(version),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	// Sampler defaults to parentbased_always_on; OTEL_TRACES_SAMPLER and
	// OTEL_TRACES_SAMPLER_ARG override it (handled by the SDK).
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	log.Info("tracing enabled", "endpoint", os.Getenv(envEndpoint), "service", service)

	return func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("telemetry: shutdown tracer provider: %w", err)
		}
		return nil
	}, nil
}
