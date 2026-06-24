package judge

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

func testSpanContext() trace.SpanContext {
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0xaa, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		SpanID:  trace.SpanID{0xbb, 1, 2, 3, 4, 5, 6, 7},
	})
}

func TestEnqueueCarriesSpanContext(t *testing.T) {
	// Not parallel: relies on the global W3C propagator (installed by default;
	// set it explicitly so the test is order-independent).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	q := &fakeQueue{}
	svc := newServiceWith(newTestStore(), &fakeExecutor{}, &fakeBroadcaster{}, q, &fakeLeaderboard{},
		Config{QueueDepthLimit: 1024, ConsumerName: "test"})

	sc := testSpanContext()
	if !svc.Enqueue(trace.ContextWithSpanContext(context.Background(), sc), uuid.New()) {
		t.Fatal("enqueue rejected below the depth limit")
	}
	// No span in context (startup/reconcile path) → empty traceparent.
	if !svc.Enqueue(context.Background(), uuid.New()) {
		t.Fatal("enqueue rejected below the depth limit")
	}

	items := q.items()
	if len(items) != 2 {
		t.Fatalf("enqueued %d items, want 2", len(items))
	}

	// The traced enqueue serializes the span into a W3C traceparent carrying
	// the same trace ID; decoding it round-trips the link.
	link := spanContextFrom(items[0].traceParent)
	if !link.IsValid() {
		t.Fatal("first item carries no usable traceparent")
	}
	if link.TraceID() != sc.TraceID() {
		t.Errorf("traceparent trace id = %s, want %s", link.TraceID(), sc.TraceID())
	}
	if items[1].traceParent != "" {
		t.Errorf("untraced enqueue carried a traceparent %q, want empty", items[1].traceParent)
	}
}

func TestProcessEmitsLinkedRootSpan(t *testing.T) {
	// Not parallel: swaps the global tracer provider.
	exporter := tracetest.NewInMemoryExporter()
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)))
	t.Cleanup(func() { otel.SetTracerProvider(old) })

	store := newTestStore()
	// The tracer is captured in New, so the service must be built after the
	// provider swap.
	svc := newService(store, &fakeExecutor{responses: []*executorv1.ExecuteResponse{accepted(10)}}, &fakeBroadcaster{})

	link := testSpanContext()
	_, _ = svc.process(context.Background(), job{id: store.sub.ID, link: link})

	var judgeSpan *tracetest.SpanStub
	for i, s := range exporter.GetSpans() {
		if s.Name == "judge.process" {
			judgeSpan = &exporter.GetSpans()[i]
			break
		}
	}
	if judgeSpan == nil {
		t.Fatal("no judge.process span exported")
	}

	// Root (the HTTP request is long gone), linked to the enqueuer.
	if judgeSpan.Parent.IsValid() {
		t.Error("judge.process has a parent; the async hand-off must be a link, not a child")
	}
	if len(judgeSpan.Links) != 1 || judgeSpan.Links[0].SpanContext.TraceID() != link.TraceID() {
		t.Errorf("judge.process links = %v, want one link back to the enqueuing request", judgeSpan.Links)
	}

	var verdict string
	for _, attr := range judgeSpan.Attributes {
		if string(attr.Key) == "arena.verdict" {
			verdict = attr.Value.AsString()
		}
	}
	if verdict != "accepted" {
		t.Errorf("arena.verdict attribute = %q, want %q", verdict, "accepted")
	}
}
