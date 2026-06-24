# ADR-0010: Observability architecture

- Status: accepted (2026-06-12)
- Phase: 6
- Related: ADR-0005/0006 (the sandbox pipeline the traces illuminate),
  ADR-0009 (deploy shapes)

## Context

The platform judges untrusted code through an async pipeline spanning two
services, a queue, gRPC, and Docker. "Is it slow because of compile times,
queueing, the daemon, or the database?" must be answerable from data, not
guesses. Phase 6 adds Prometheus metrics, OpenTelemetry tracing, and Grafana
dashboards — with the constraint that the platform must keep working (and
stay fast) when no observability backend exists.

## Decisions

### 1. prometheus/client_golang for metrics, OTel SDK for traces

One instrumentation API (OTel for both signals) is tempting, but OTel
metrics through a Prometheus exporter adds a registry bridge and a naming
translation for zero functional gain when Prometheus is the known backend.
Each signal uses its dominant, stable standard. Revisit only if an
OTLP-native metrics backend ever matters.

### 2. Tracing is fail-soft and off by default

No `OTEL_EXPORTER_OTLP_ENDPOINT` → no-op tracer provider, one startup log
line, zero goroutines (`pkg/telemetry.Init`). Export errors are logged,
never propagated. The W3C propagator is installed even when tracing is off
so foreign trace context still flows through untouched. Instrumentation
must never be the thing that takes the platform down.

### 3. The async judge hop is a span LINK, not a parent/child

The submit request returns 202 long before judging starts; pretending the
judge span is a child of the HTTP span would misrepresent causality and
produce hours-long "requests". The queue carries the enqueuer's
`trace.SpanContext`; `judge.process` starts a new root linked to it.
Startup-requeued jobs have no link — an honest signal of "recovered after
restart". Trade-off: some UIs render links less prominently than parents;
Tempo handles them.

### 4. Dedicated metrics listeners (:9100 gateway, :9101 executor)

`/metrics` never rides the public listener: the gateway's port is exposed
via NodePort, and operational internals (route shapes, error rates, pool
stats) are reconnaissance gold. In Kubernetes the metrics ports get their
own ClusterIP Services — adding them to the NodePort Service would assign
them node ports.

### 5. Cardinality rule: labels from closed sets only

Route labels use the Go 1.22 mux pattern (`req.Pattern`), with unmatched
requests collapsed into one `unmatched` series. No user/contest/submission
IDs in any label (IDs belong on span attributes). WebSocket upgrades are
counted but get no span and no duration observation — connection-lifetime
spans and hour-scale histogram samples are noise.

### 6. Stack: three pinned official containers, config single-sourced

Prometheus + Tempo + Grafana (~0.6 GB) instead of kube-prometheus-stack
(CRDs, operators, >1 GB — rejected for a 6 GB dev VM) or the otel-lgtm
all-in-one (hides exactly the config this repo exists to demonstrate).
Dashboards, alert rules, Tempo config and Grafana provisioning live once in
`infra/helm/arena/files/observability/` — compose bind-mounts them, the
chart ships them as ConfigMaps via `.Files.Get`. Only the Prometheus scrape
config is per-shape (compose scrapes `host.docker.internal`; the cluster
flavor needs release-prefixed Service DNS and is templated).

The in-cluster stack's Services use fixed names (`prometheus`, `tempo`,
`grafana`) so the shared Grafana datasource file works in both shapes; the
stack is a singleton per namespace and documented as such.

### 7. In-cluster stack default-off; scraping via static Service targets

`observability.enabled=false` by default: Kind + DinD already runs near the
dev machine's 6 GB ceiling (the services always expose /metrics regardless
— collection is what's optional). Prometheus scrapes the two metrics
Services statically; pod-level service discovery needs RBAC and only pays
off with replicas > 1 — that is Phase 7 work alongside scaling.

### 8. Logs correlate by trace_id; no log shipping

`pkg/logging` wraps every handler with a `trace_id`/`span_id` decorator
driven by the request context — every existing `*Context` log call gained
correlation with zero call-site changes. Shipping logs (Loki) adds infra
weight without new lessons; grep + trace_id covers the dev loop. Documented
follow-up, not scope.

## Consequences

- One trace shows HTTP → judge (linked) → gRPC → sandbox phases
  (create/write_source/compile/cleanup/run), with phase durations doubling
  as Prometheus histograms (`arena_executor_phase_duration_seconds`).
- The metric catalog (phase 6 doc §2) is the contract; dashboards and alert
  rules are code, reviewed like code.
- Judge latency overhead measured at the gate: span creation is nanoseconds
  against a 1.5–2.6 s judge path (<1%).
- Two instrumentation dependencies enter the tree (client_golang, OTel SDK
  + contrib otelgrpc/otelpgx/redisotel), pinned together — contrib versions
  must move with the SDK.
