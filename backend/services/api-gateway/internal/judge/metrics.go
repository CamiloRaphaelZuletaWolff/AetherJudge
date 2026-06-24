package judge

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	queueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "arena_judge_queue_depth",
		Help: "Submissions waiting in the judge queue (backpressure early warning).",
	})

	jobsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "arena_judge_jobs_total",
		Help: "Judged submissions by final verdict (internal_error = judge-side failures).",
	}, []string{"verdict"})

	judgeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "arena_judge_duration_seconds",
		Help: "End-to-end judging latency (dequeue to verdict persisted) by language.",
		// Judging is seconds-scale (compile + sandboxed runs), so the
		// default web-latency buckets would lump everything together.
		Buckets: []float64{0.5, 1, 2, 4, 8, 16, 32, 64},
	}, []string{"language"})
)
