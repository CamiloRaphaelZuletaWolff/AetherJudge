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
