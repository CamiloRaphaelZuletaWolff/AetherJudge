# Arena — frontend

Next.js 15 (App Router) + React 19 + TypeScript (strict) + Tailwind CSS v4.

This package is the web client for Arena; project-wide documentation lives in
the [repository root](../README.md) and [docs/](../docs/).

## Commands

```bash
pnpm install        # install dependencies
pnpm dev            # dev server with hot reload → http://localhost:3000
pnpm build          # production build
pnpm lint           # ESLint
pnpm typecheck      # tsc --noEmit
pnpm test           # Vitest (jsdom + Testing Library)
pnpm test:ui        # browser UX checks with fixture API/WS data (no backend required)
pnpm format         # Prettier (write)
pnpm format:check   # Prettier (verify only)
```

```bash
pnpm test:e2e       # Playwright journeys through the real stack
                    # (needs: task infra:up && task db:seed && task executor:images)
```

## Structure

```
src/app/            routes: / · /auth · /dashboard · /room/[id]
src/components/     ui kit, auth forms + gate, dashboard cards, room panels
src/hooks/          use-contest-events (WS → Query cache, backoff reconnect)
src/lib/            api.ts (token + single-flight refresh), schemas.ts (Zod
                    boundary), query-keys.ts, format.ts
src/stores/         auth (identity, in-memory token), editor (persisted drafts)
e2e/                Playwright specs (the Track-1 acceptance journeys)
```

Two rules worth knowing before editing:

- Access tokens are **never stored** (memory only); the refresh cookie does
  session recovery. Don't add a second refresh path — the backend's rotation
  reuse-detection will revoke the session family.
- Every REST/WS payload passes through `lib/schemas.ts`. New endpoint = new
  schema, parsed at the boundary.

## Scoreboard and interaction behavior

The scoreboard keeps the server's authoritative ranks. A score update first
highlights the competitor in their old position for 280 ms, then moves the
same row to its new rank while the other rows make room. Updates received
during a move are coalesced into the latest snapshot, so bursts cannot create
an unbounded animation queue. Initial snapshots do not celebrate old solves.

Pause freezes the displayed standings while live data continues arriving.
Resume catches up to the latest snapshot. The animation toggle and the OS
reduced-motion preference apply updates immediately. Searches retain actual
contest ranks, and status announcements describe gains to screen readers.

The contest room can expand the scoreboard without unmounting the editor.
Code drafts remain per problem and language. The basic editor is available
if Monaco cannot load. Upcoming and finished contests explain why submission
is disabled, while code editing and custom runs remain available.

`pnpm test:ui` starts the frontend on port 3100 and runs deterministic browser
checks for row movement, pause/resume, mobile overflow, keyboard navigation,
draft preservation, account forms, and reduced motion. Install the browser
once with `pnpm exec playwright install chromium`. Screenshots and traces are
written to the ignored `test-results/` directory. These tests complement the
full-stack `pnpm test:e2e` journeys; they do not exercise the real judge.
