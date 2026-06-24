package realtime

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// wsActiveConnections counts live room connections process-wide. No per-room
// label on purpose: contest IDs are an unbounded set and would mint a series
// per contest (the cardinality rule from the phase 6 catalog).
var wsActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "arena_ws_active_connections",
	Help: "Open WebSocket room connections.",
})
