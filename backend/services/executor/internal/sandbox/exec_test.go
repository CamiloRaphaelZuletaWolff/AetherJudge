package sandbox

import (
	"strings"
	"testing"
)

func TestCappedBufferUnderLimit(t *testing.T) {
	t.Parallel()

	b := newCappedBuffer(10)
	n, err := b.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("Write = (%d, %v), want (5, nil)", n, err)
	}
	if b.String() != "hello" || b.truncated {
		t.Errorf("got %q (truncated=%v), want %q untruncated", b.String(), b.truncated, "hello")
	}
}

func TestCappedBufferTruncatesAtLimit(t *testing.T) {
	t.Parallel()

	b := newCappedBuffer(4)
	if n, err := b.Write([]byte("abcdef")); err != nil || n != 6 {
		t.Fatalf("Write = (%d, %v), want full length (6, nil) so the stream keeps draining", n, err)
	}
	if b.String() != "abcd" || !b.truncated {
		t.Errorf("got %q (truncated=%v), want %q truncated", b.String(), b.truncated, "abcd")
	}
}

func TestCappedBufferDiscardsAfterLimit(t *testing.T) {
	t.Parallel()

	b := newCappedBuffer(3)
	for range 100 {
		if n, err := b.Write([]byte("xyz")); err != nil || n != 3 {
			t.Fatalf("Write = (%d, %v), want (3, nil)", n, err)
		}
	}
	if b.String() != "xyz" {
		t.Errorf("buffer = %q, want %q", b.String(), "xyz")
	}
	if int64(len(b.String())) != 3 {
		t.Errorf("buffer grew past limit: %d bytes", len(b.String()))
	}
}

func TestCappedBufferLargeFlood(t *testing.T) {
	t.Parallel()

	const limit = 1024
	b := newCappedBuffer(limit)
	chunk := []byte(strings.Repeat("A", 8192))
	for range 1000 { // ~8 MB flood
		if _, err := b.Write(chunk); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if len(b.String()) != limit {
		t.Errorf("captured %d bytes, want exactly %d", len(b.String()), limit)
	}
	if !b.truncated {
		t.Error("truncated flag not set after flood")
	}
}
