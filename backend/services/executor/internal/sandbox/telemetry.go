package sandbox

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// phaseDuration answers "where does judge time go" — compile dominates cold
// caches, run dominates TLE-bound submissions, create/remove expose Docker
// daemon health.
var phaseDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "arena_executor_phase_duration_seconds",
	Help:    "Sandbox pipeline phase latency by phase and language.",
	Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 4, 8, 16, 32},
}, []string{"phase", "language"})

var tracer = otel.Tracer("github.com/caezu/arena/backend/services/executor/internal/sandbox")

// startPhase opens a pipeline-phase span and starts its duration clock. The
// returned func must be called exactly once when the phase ends; it records
// the histogram observation and closes the span (marked failed when err is
// non-nil).
func startPhase(ctx context.Context, phase, language string) (context.Context, func(err error)) {
	ctx, span := tracer.Start(ctx, "sandbox."+phase)
	start := time.Now()
	return ctx, func(err error) {
		phaseDuration.WithLabelValues(phase, language).Observe(time.Since(start).Seconds())
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}
