# ADR-0004: PostgreSQL as the only source of truth; Redis as ephemeral infrastructure

- Status: accepted
- Date: 2026-06-11
- Phase: 1

## Context

Arena needs durable storage (users, contests, problems, submissions,
standings) and low-latency shared state (live leaderboards, WebSocket fan-out
across gateway replicas, rate-limit counters). Both PostgreSQL and Redis will
be deployed; the question is where the line between them sits, because a
blurry line is how systems end up with data that exists only in a cache.

## Decision

- **PostgreSQL owns all durable state.** Schema is migrations-first with
  foreign keys and constraints (Phase 3).
- **Redis holds only state that can be rebuilt from PostgreSQL**: Pub/Sub
  channels for WebSocket synchronization, sorted sets for live leaderboard
  ranking, token-bucket counters for rate limiting.
- Operating rule: **losing Redis must never lose data.** Any design that
  would violate this is wrong by definition.

## Rationale

- Competitive-programming verdicts and standings are the product; they need
  transactions, constraints and durability — relational territory.
- Live leaderboards need ordered reads at interactive latency under fan-out;
  Redis sorted sets are the canonical tool.
- A crisp rule ("rebuildable or it doesn't go in Redis") is enforceable in
  review and prevents drift toward cache-as-database.

## Consequences

- Leaderboard state is written twice (PostgreSQL for truth, Redis for speed);
  the rebuild path from PostgreSQL must exist and be tested (Phase 3/7).
- Local and production Redis run without persistence (no RDB/AOF), which
  makes restarts trivially safe and encodes the rule in infrastructure.
