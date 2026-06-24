// Package logging provides the shared slog configuration used by every
// Arena service, so log output stays uniform across the platform.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Format controls the output encoding of a logger.
type Format string

const (
	// FormatJSON emits one JSON object per line; the production default.
	FormatJSON Format = "json"
	// FormatText emits human-readable key=value lines for local development.
	FormatText Format = "text"
)

// New builds a *slog.Logger writing to w.
//
// level accepts "debug", "info", "warn" or "error"; format accepts "json" or
// "text" (both case-insensitive). Unrecognized values return an error so that
// configuration typos fail at startup instead of silently misbehaving.
func New(w io.Writer, level, format string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	switch Format(strings.ToLower(format)) {
	case FormatJSON:
		handler = slog.NewJSONHandler(w, opts)
	case FormatText:
		handler = slog.NewTextHandler(w, opts)
	default:
		return nil, fmt.Errorf("logging: unknown format %q (want %q or %q)", format, FormatJSON, FormatText)
	}

	// Correlate logs with traces: records written inside a span carry its
	// trace_id/span_id (no-op outside spans or with tracing disabled).
	return slog.New(traceHandler{inner: handler}), nil
}

func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: unknown level %q (want debug, info, warn or error)", level)
	}
}
