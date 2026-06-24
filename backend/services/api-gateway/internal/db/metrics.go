package db

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RegisterPoolMetrics exposes pgxpool statistics on the default Prometheus
// registry. Call it exactly once, from main, after Connect — promauto
// panics on duplicate registration by design, which keeps a second
// accidental call from silently shadowing the first pool.
func RegisterPoolMetrics(pool *pgxpool.Pool) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "arena_pgx_pool_acquired_conns",
		Help: "Connections currently checked out of the pool.",
	}, func() float64 { return float64(pool.Stat().AcquiredConns()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "arena_pgx_pool_idle_conns",
		Help: "Idle connections sitting in the pool.",
	}, func() float64 { return float64(pool.Stat().IdleConns()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "arena_pgx_pool_total_conns",
		Help: "Total connections held by the pool (idle + acquired + constructing).",
	}, func() float64 { return float64(pool.Stat().TotalConns()) })

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "arena_pgx_pool_max_conns",
		Help: "Configured pool ceiling; acquired == max means saturation.",
	}, func() float64 { return float64(pool.Stat().MaxConns()) })

	promauto.NewCounterFunc(prometheus.CounterOpts{
		Name: "arena_pgx_pool_acquires_total",
		Help: "Cumulative successful connection acquires.",
	}, func() float64 { return float64(pool.Stat().AcquireCount()) })

	promauto.NewCounterFunc(prometheus.CounterOpts{
		Name: "arena_pgx_pool_empty_acquires_total",
		Help: "Acquires that had to wait because the pool was empty (contention signal).",
	}, func() float64 { return float64(pool.Stat().EmptyAcquireCount()) })
}
