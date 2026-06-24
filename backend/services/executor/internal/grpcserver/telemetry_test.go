package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

// TestTraceContextPropagatesAcrossGRPC proves the cross-service half of the
// phase 6 trace: a client span created on the gateway side arrives at the
// executor handler as the same trace (otelgrpc client handler → W3C
// metadata → otelgrpc server handler installed by New).
func TestTraceContextPropagatesAcrossGRPC(t *testing.T) {
	// Not parallel: swaps the global tracer provider and propagator.
	exporter := tracetest.NewInMemoryExporter()
	oldTP := otel.GetTracerProvider()
	oldProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTP)
		otel.SetTextMapPropagator(oldProp)
	})

	var serverSC trace.SpanContext
	stub := &stubExecutor{
		execute: func(ctx context.Context, _ *executorv1.ExecuteRequest) (*executorv1.ExecuteResponse, error) {
			serverSC = trace.SpanContextFromContext(ctx)
			return &executorv1.ExecuteResponse{Verdict: executorv1.Verdict_VERDICT_ACCEPTED}, nil
		},
	}

	lis := bufconn.Listen(1 << 20)
	srv := New("unused", discardLogger(), stub)
	srvCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(srvCtx, lis) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Serve did not return after cancellation")
		}
	})

	// Mirror judge.Dial: the production client carries the otelgrpc handler.
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close conn: %v", err)
		}
	})

	tracer := otel.Tracer("test")
	ctx, parent := tracer.Start(context.Background(), "judge.process")
	if _, err := executorv1.NewExecutorServiceClient(conn).Execute(ctx, validRequest()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	parent.End()

	if !serverSC.IsValid() {
		t.Fatal("executor handler saw no span context")
	}
	// The handler sees the SERVER span otelgrpc opened — a local span on
	// the caller's trace. Same trace ID = propagation worked end to end.
	if got, want := serverSC.TraceID(), parent.SpanContext().TraceID(); got != want {
		t.Errorf("executor trace id = %s, want the caller's %s", got, want)
	}
}

func validRequest() *executorv1.ExecuteRequest {
	return &executorv1.ExecuteRequest{
		Language: executorv1.Language_LANGUAGE_PYTHON,
		Code:     "print(1)",
	}
}
