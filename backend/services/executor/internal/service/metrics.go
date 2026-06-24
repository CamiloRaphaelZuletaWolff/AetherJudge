package service

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

var (
	executionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "arena_executor_executions_total",
		Help: "Completed executions by language and verdict.",
	}, []string{"language", "verdict"})

	activeExecutions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "arena_executor_active_executions",
		Help: "Executions currently holding a sandbox slot (max = EXECUTOR_MAX_CONCURRENT).",
	})

	admissionWait = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "arena_executor_admission_wait_seconds",
		Help:    "Time spent queued for a sandbox slot before execution starts.",
		Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 30},
	})
)

// verdictLabel turns the proto enum name into the platform's verdict strings
// ("VERDICT_WRONG_ANSWER" → "wrong_answer") so executor and gateway metrics
// share one vocabulary.
func verdictLabel(v executorv1.Verdict) string {
	return strings.ToLower(strings.TrimPrefix(v.String(), "VERDICT_"))
}
