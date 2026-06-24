# ADR-0003: REST + WebSocket externally, gRPC internally, contracts via buf

- Status: accepted
- Date: 2026-06-11
- Phase: 1

## Context

The browser needs an API for auth/contest/submission flows and a live channel
for leaderboard updates. The gateway needs to invoke the executor with a
typed, evolvable contract. The executor call is on the platform's most
security- and latency-sensitive path.

## Decision

- Browser ↔ gateway: **REST (JSON)** for request/response, **WebSocket** for
  live updates.
- Gateway ↔ executor: **gRPC**, with contracts defined in
  `backend/proto` and compiled by **buf**.
- Generated Go code is **committed** to `backend/pkg/pb`; CI regenerates and
  fails on drift.

## Rationale

- REST + WebSocket are native to browsers — no gRPC-Web proxy layer for a
  public API that only this frontend consumes.
- gRPC gives the internal boundary a schema: breaking changes are caught by
  `buf breaking`, and the request/response messages double as queue payloads
  when Phase 7 introduces asynchronous execution.
- buf over raw `protoc`: built-in compiler (no separate C++ binary to
  install — relevant for Windows contributors), proto linting, and
  breaking-change detection as CI gates.
- Committing generated code means `go build` works with zero proto tooling
  installed, and reviewers see exactly what the contract compiles to.

## Consequences

- Generated-code diffs add noise to contract PRs; acceptable for one proto
  package, revisit if contracts multiply.
- Anyone editing `.proto` files needs the buf toolchain (`task tools:install`);
  CI's drift check makes forgetting to regenerate a loud failure, not a
  silent inconsistency.
