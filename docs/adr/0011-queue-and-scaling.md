# ADR-0011: Durable judge queue and horizontal scaling

- Status: accepted (2026-06-17)
- Phase: 7
- Related: ADR-0004 (PG truth / Redis rebuildable), ADR-0008 (per-room
  pub/sub), ADR-0009 (deploy shapes), ADR-0012 (leaderboard read path)

## Context

Through Phase 6 the gateway judged submissions with an **in-process Go
channel**: the HTTP handler pushed a submission ID onto `chan job`, a pool of
worker goroutines drained it, and crash recovery was a startup re-enqueue of
every unfinished row. This cannot scale past one replica — each replica has
its own channel, and the startup requeue would make every replica re-judge
all pending work — and it couples "accept a submission" to "judge a
submission" inside one process. Phase 7's goal is to scale judging
horizontally and prove it.

## Decisions

### 1. Redis Streams consumer group as the judge queue

The in-process channel becomes a Redis Stream (`arena:judgeq`) consumed by a
group (`judges`). This gives at-least-once delivery, per-message acks, and —
critically — lets many consumers across many processes share one queue
safely. The proto was designed request/response-shaped for exactly this
(ADR-0003); no executor change was needed.

Rejected alternatives: **PostgreSQL `SELECT … FOR UPDATE SKIP LOCKED`**
(couples judge throughput to the primary DB and reuses the OLTP store as a
broker); an **external broker** (RabbitMQ/NATS/Kafka — violates the
stdlib-first, minimal-infra invariant for no gain at this scale).

### 2. PostgreSQL stays the source of truth; the stream is rebuildable

A queued submission is a `submissions` row (`status='queued'`); the stream is
only dispatch. **Losing Redis loses no data** (ADR-0004): on startup a
reconciler re-enqueues unfinished rows when the stream is empty (the
Redis-was-flushed signal). Routine crashes are handled separately by claim
(below), so the reconciler does not fight live consumers.

### 3. At-least-once ⇒ idempotent judging

Redelivered or reclaimed messages must not double-judge or double-score.
Guarantees: `process` returns early when `status='done'`; scoring records the
solve with `ON CONFLICT DO NOTHING` and reads back the standing. A
redelivered, already-judged submission is a no-op. (Tested:
`TestDuplicateDeliveryScoresOnce`.)

### 4. Crash recovery by claim; poison messages dead-lettered

A consumer that dies mid-judge leaves its message un-acked. A reclaim loop
(`XPENDING` + `XCLAIM`) transfers messages idle beyond `ClaimMinIdle` to a
live consumer. A message whose delivery count exceeds `MaxDeliveries`
(it keeps crashing its consumer) is **dead-lettered**: the submission is
marked `internal_error` (visible, never silently lost) and acked.
Infrastructure failures (executor/DB unreachable) are returned as *retryable*
— the submission is left un-acked for a later attempt rather than burned as a
verdict, which the old synchronous path could not do.

### 5. Web/worker split

Judging runs in a dedicated `cmd/worker` deployment that scales independently
of the HTTP gateway (`JUDGE_WORKERS=0` on the web tier). Same image, three
entrypoints (`/server`, `/seed`, `/worker`). For single-node local dev the
gateway can still run consumers in-process (`JUDGE_WORKERS>0`) — the consumer
group makes both shapes correct. This is the textbook reason a queue exists:
decouple accept-capacity from judge-capacity.

### 6. Executor fan-out: StatefulSet + headless Service + gRPC round-robin

The executor was a single-replica Deployment with one RWO DinD PVC (the PVC
being exactly what blocked `replicas>1`). It becomes a **StatefulSet** with
`volumeClaimTemplates` (a per-pod DinD store) behind a **headless Service**.
The gateway/worker dial `dns:///…-executor:9090` with a `round_robin` service
config, so Execute calls spread across all executor pods. Judge throughput
now scales as *consumers × executor replicas × MaxConcurrent*.

### 7. Per-replica scraping via DNS-SD; CPU-based HPA

Headless metrics Services let the in-cluster Prometheus discover every pod
via `dns_sd_configs` (no Kubernetes API/RBAC). An optional CPU-based HPA
(default off) scales the worker and executor. Honest limit: judging is
Docker/IO-bound, so CPU is an imperfect signal and a single node bounds real
gains — a queue-depth-driven HPA (custom-metrics adapter) is the production
refinement, out of scope here.

## Consequences

- Multiple gateway/worker replicas share one queue safely; a worker can die
  mid-judge and another finishes the job; a flushed Redis is reconciled from
  PG. The backpressure path (`XLEN` ≥ limit → HTTP 503) is preserved.
- One more durable-ish dependency surface on Redis (queue), but every Redis
  role remains rebuildable from PG (ADR-0004). Redis HA stays out of scope
  and documented.
- Measured locally with the k6 harness ([`loadtest/`](../../loadtest/)) and
  read through the Phase 6 dashboards: submit latency stays in the
  milliseconds while judging absorbs the burst (sandbox-bound), the queue
  drains to empty, and `Execute` fans out evenly across executor replicas
  (verified N=1→2 — ~13/14 of 27 calls per pod via gRPC round-robin).
