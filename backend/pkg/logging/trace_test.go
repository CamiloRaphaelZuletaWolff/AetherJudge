package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandlerAddsIDsInsideSpan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:  trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	log.InfoContext(ctx, "judging")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if got, want := record["trace_id"], sc.TraceID().String(); got != want {
		t.Errorf("trace_id = %v, want %v", got, want)
	}
	if got, want := record["span_id"], sc.SpanID().String(); got != want {
		t.Errorf("span_id = %v, want %v", got, want)
	}
}

func TestTraceHandlerOmitsIDsOutsideSpan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	log.InfoContext(context.Background(), "no span here")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if _, present := record["trace_id"]; present {
		t.Errorf("trace_id present on a record logged outside a span: %s", buf.String())
	}
}
