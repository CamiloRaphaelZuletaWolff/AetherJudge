# ADR-0008: In-process judge workers with startup requeue; per-room Redis Pub/Sub fan-out

- Status: accepted
- Date: 2026-06-11
- Phase: 3

## Context

Submissions must be judged asynchronously (a REST request can't wait tens of
seconds for compile+run across test cases), and every client in a contest
room must see submission and leaderboard changes immediately — including
clients connected to *different* gateway replicas, once there are several.

## Decision

### Judging

A bounded in-process queue feeds `JUDGE_WORKERS` goroutines. Each worker
drives the executor over gRPC once per test case (stopping at the first
non-accepted verdict), persists the result, scores accepts transactionally
(ICPC-lite: solved count, then penalty = solve time + 20 min per prior wrong
attempt; compilation errors don't count), and publishes events.

Honesty about durability: an in-process queue dies with the process. Two
mitigations make that acceptable for the MVP:

1. Submissions are persisted as `queued` *before* enqueueing, so nothing is
   ever known only to the queue.
2. On startup the gateway re-enqueues everything not yet `done`.

A full enqueue returns HTTP 503 (backpressure) rather than blocking the
handler. The durable queue (and horizontal judge scaling) is Phase 7 by
design, not by accident.

Failures during judging — executor unreachable, missing test cases — yield
the `internal_error` verdict, visibly retryable, never a fabricated judgment
and never a silently stuck submission.

### Realtime

Each contest room maps to one Redis Pub/Sub channel
(`contest:<id>:events`). Every gateway replica holds **one subscription per
room with local clients** and fans messages out to its own WebSocket
connections. Publishers (judge workers, join handler) publish to Redis only —
they never touch the hub directly, so events reach all replicas identically
(this is the property that makes Phase 7 horizontal scaling a deployment
change, not a redesign).

Hub rules:

- Per-client send buffers are bounded; a client that can't drain is
  disconnected (it reconnects and re-snapshots via REST) instead of
  back-pressuring the room.
- WS is **delta, not source of truth**: clients fetch current state via REST
  on join; missing an event can cost freshness, never correctness.
- Leaderboard standings ride events from the indexed PostgreSQL query; the
  Redis ZSET read-path optimization is deliberately deferred to Phase 7
  (where the spec schedules it) rather than built speculatively.

## Consequences

- Judge throughput per replica is `JUDGE_WORKERS` × executor concurrency;
  fine for MVP loads, measured properly in Phase 7.
- Events may arrive before a subscriber attaches (e.g. instant verdicts);
  the REST-snapshot-on-join pattern absorbs this by construction.
- The integration suite proves the acceptance property end to end: two
  WebSocket clients in one room both observe a third party's submission
  lifecycle and the resulting leaderboard update, with the executor stubbed
  over real gRPC.
