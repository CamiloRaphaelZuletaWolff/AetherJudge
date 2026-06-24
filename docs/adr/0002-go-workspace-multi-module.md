# ADR-0002: Go workspace with one module per service plus a shared module

- Status: accepted
- Date: 2026-06-11
- Phase: 1

## Context

The backend has two services (`api-gateway`, `executor`) and shared code
(generated protobuf bindings, logging setup). Go offers two ways to organize
this: one module for the whole backend, or one module per service tied
together with a `go.work` workspace.

## Decision

Three modules — `backend/pkg`, `backend/services/api-gateway`,
`backend/services/executor` — joined by `backend/go.work`.

Cross-module dependencies use `require` plus a relative `replace` directive
(e.g. the gateway requires `backend/pkg` and replaces it with `../../pkg`).

## Rationale

- **Independent dependency trees.** The executor needs the Docker SDK
  (Phase 2); the gateway must never inherit that dependency surface. A single
  module would merge both into one `go.sum`, inflating every service's
  supply-chain exposure and Docker build context.
- **Honest service boundaries.** A service cannot silently import another
  service's internals; anything shared must be promoted into `pkg`
  deliberately.
- **Standalone buildability.** Because each `go.mod` carries an explicit
  `replace`, every module builds without the workspace file — which is what
  per-service Dockerfiles (Phase 5) and module-scoped CI rely on.

## Consequences

- Three `go.mod`/`go.sum` files to maintain; dependency upgrades run per
  module. Acceptable at this scale.
- Tooling must iterate over modules (`go -C <module> test ./...`); the
  Taskfile encodes the module list once.
- The `v0.0.0` + `replace` pattern means modules are not independently
  publishable. They are not meant to be.
