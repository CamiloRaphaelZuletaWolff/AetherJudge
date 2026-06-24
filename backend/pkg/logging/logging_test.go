package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewValidConfigurations(t *testing.T) {
	t.Parallel()

	for _, level := range []string{"debug", "info", "warn", "error", "INFO", "Error"} {
		for _, format := range []string{"json", "text", "JSON"} {
			log, err := New(&bytes.Buffer{}, level, format)
			if err != nil {
				t.Errorf("New(level=%q, format=%q) returned error: %v", level, format, err)
			}
			if log == nil {
				t.Errorf("New(level=%q, format=%q) returned nil logger", level, format)
			}
		}
	}
}

func TestNewRejectsUnknownLevel(t *testing.T) {
	t.Parallel()

	if _, err := New(&bytes.Buffer{}, "verbose", "json"); err == nil {
		t.Fatal("New with unknown level returned nil error, want error")
	}
}

func TestNewRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	if _, err := New(&bytes.Buffer{}, "info", "xml"); err == nil {
		t.Fatal("New with unknown format returned nil error, want error")
	}
}

func TestJSONOutputIsStructured(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	log.Info("hello", "service", "test")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if record["msg"] != "hello" {
		t.Errorf("msg = %v, want %q", record["msg"], "hello")
	}
	if record["service"] != "test" {
		t.Errorf("service = %v, want %q", record["service"], "test")
	}
}

func TestLevelFiltering(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log, err := New(&buf, "warn", "text")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	log.Info("should be filtered")
	log.Warn("should appear")

	out := buf.String()
	if strings.Contains(out, "should be filtered") {
		t.Errorf("info record leaked through warn-level logger: %s", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Errorf("warn record missing from output: %s", out)
	}
}
