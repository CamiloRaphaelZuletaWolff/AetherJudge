package redisx

import (
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestItemsFromMessagesParsesFields(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	msgs := []redis.XMessage{{
		ID:     "1-0",
		Values: map[string]any{fieldSubmissionID: id.String(), fieldTraceParent: "tp-abc"},
	}}

	items := itemsFromMessages(msgs)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].SubmissionID != id {
		t.Errorf("submission id = %s, want %s", items[0].SubmissionID, id)
	}
	if items[0].MessageID != "1-0" {
		t.Errorf("message id = %q, want 1-0", items[0].MessageID)
	}
	if items[0].TraceParent != "tp-abc" {
		t.Errorf("traceparent = %q, want tp-abc", items[0].TraceParent)
	}
}

func TestItemsFromMessagesToleratesBadID(t *testing.T) {
	t.Parallel()

	// A malformed entry must not panic and must surface a zero submission ID so
	// the consumer can ack-and-drop it rather than wedge the group.
	msgs := []redis.XMessage{{ID: "9-0", Values: map[string]any{fieldSubmissionID: "not-a-uuid"}}}

	items := itemsFromMessages(msgs)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].SubmissionID != uuid.Nil {
		t.Errorf("submission id = %s, want Nil for an unparseable entry", items[0].SubmissionID)
	}
	if items[0].MessageID != "9-0" {
		t.Errorf("message id = %q, want 9-0 (so it can be acked-and-dropped)", items[0].MessageID)
	}
}
