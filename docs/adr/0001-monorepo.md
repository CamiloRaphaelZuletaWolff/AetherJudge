# ADR-0001: Monorepo for all services, frontend, and infrastructure

- Status: accepted
- Date: 2026-06-11
- Phase: 1

## Context

Arena consists of two Go services, a Next.js frontend, protobuf contracts
shared between services, infrastructure definitions, and documentation. We
need to decide how to organize these across repositories.

## Decision

A single repository with top-level `backend/`, `frontend/`, `infra/`, and
`docs/` directories.

## Rationale

- **Atomic changes.** A change to the executor's proto contract, its
  implementation, the gateway's client call, and the docs lands as one
  reviewable commit. With multiple repos this becomes a coordination problem
  (versioned contract packages, lockstep releases) that a two-service system
  does not deserve.
- **One CI pipeline, one toolchain story.** Contributors set up once.
- **Single source of truth for docs.** ADRs and architecture docs sit next to
  the code they describe.

## Consequences

- CI must stay fast as the repo grows; jobs are split per area (backend,
  proto, frontend) so failures are attributable. If build times degrade,
  path-based filtering is the next step — deliberately not added until it
  pays for its complexity.
- Git history mixes concerns; conventional-commit scopes (`gateway:`,
  `executor:`, `frontend:`, `infra:`, `docs:`) keep it navigable.
