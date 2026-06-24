# Load testing (k6)

`submit.js` drives the real submission path — signup → submit → poll until
judged — at a configurable concurrency, and reports judge latency. It is the
harness used to measure the queue + executor scaling behavior described in
[ADR-0011](../docs/adr/0011-queue-and-scaling.md).

## Prerequisites

- [k6](https://k6.io) — `winget install k6.k6` (Windows) /
  `brew install k6` (macOS) / [other installs](https://grafana.com/docs/k6/latest/set-up/install-k6/).
- A running stack (compose dev path or the Kind cluster).
- **Generous rate limits** — this measures judging throughput, not
  rate-limiting, so run the gateway/worker with:
  ```
  RATE_LIMIT_AUTH_PER_MIN=100000 RATE_LIMIT_SUBMIT_PER_MIN=100000
  ```

## Run

```bash
# Local dev stack (gateway on :8080)
task loadtest

# Explicit target + concurrency + steady-state duration
ARENA_URL=http://localhost:8080 VUS=25 DURATION=2m k6 run loadtest/submit.js

# Against the Kind cluster (NodePort)
ARENA_URL=http://localhost:8091 k6 run loadtest/submit.js

# Stream results into the Phase 6 Prometheus → visible on Grafana
K6_PROMETHEUS_RW_SERVER_URL=http://localhost:59090/api/v1/write \
  k6 run -o experimental-prometheus-rw loadtest/submit.js
```

## What to watch

While it runs, open the **Judging Pipeline** Grafana dashboard
(`task obs:up`, http://localhost:53000): queue depth, judged-by-verdict,
sandbox phase p95, and admission wait tell you whether you are executor-bound
(scale `executor.replicas`) or worker-bound (scale `worker.replicas`).

## Tunables (env)

| Var | Default | Meaning |
| --- | --- | --- |
| `ARENA_URL` | `http://localhost:8080` | gateway base URL |
| `VUS` | `10` | peak virtual users (concurrent submitters) |
| `DURATION` | `1m` | steady-state hold at peak |

Thresholds (the run fails if breached): `http_req_failed < 5%`,
`judge_latency_ms p95 < 90s`, `verdict_accepted > 95%`.
