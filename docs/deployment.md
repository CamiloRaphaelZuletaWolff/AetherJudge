# Deployment

Arena is built for Kubernetes (see the [Helm chart](../infra/helm/arena/) and
[ADR-0009](adr/0009-kubernetes-deployment.md)), but the API and frontend also
run well on managed PaaS. This documents a real, low-cost deployment:

- **api-gateway** → Render (free Docker web service, judges in-process)
- **Redis** → Render Key Value (free, internal-only)
- **PostgreSQL** → Neon (free; Render's free Postgres expires after 30 days)
- **frontend** → Vercel
- **executor** → *not* a PaaS web service — it needs a privileged Docker daemon
  to sandbox untrusted code, so it runs on Kubernetes or a VM (see below)

## Topology

```
 Vercel (Next.js)  ──REST/WS──▶  Render: api-gateway ──▶ Neon Postgres
        │                              │            ├──▶ Render Key Value (Redis)
        └────────── browser ──────────┘            └──▶ executor VM (gRPC over TLS
                                                         via Caddy, bearer token)
```

## Why the executor is special

The executor launches sibling Docker containers with `CapDrop: ALL`, no
network, a read-only rootfs, and resource quotas (ADR-0005). That requires
access to a Docker daemon with privileged capabilities — which PaaS web
services (Render, Vercel, most of Heroku-likes) deliberately do **not** grant.
So judging runs where we control the daemon:

- **Kubernetes** (recommended): the Helm chart deploys the executor as a
  StatefulSet with a privileged Docker-in-Docker sidecar — the only privileged
  container in the system (ADR-0009). Point the gateway's `EXECUTOR_ADDR` at the
  in-cluster service.
- **A VM** with Docker installed (`infra/vm/`): the executor and Caddy run
  under Docker Compose, pulling multi-arch images that
  `.github/workflows/executor-images.yml` publishes to GHCR. Caddy terminates
  TLS for `<ip-with-dashes>.sslip.io` with an automatic Let's Encrypt
  certificate and proxies gRPC (h2c) to the executor, whose port is bound to
  loopback only. Every Execute call must carry `authorization: Bearer
  <EXECUTOR_AUTH_TOKEN>` (constant-time check; health checks stay open). The
  gateway sets `EXECUTOR_ADDR=<host>.sslip.io:443`, `EXECUTOR_TLS=true` and the
  same token. The step-by-step guide (Oracle Cloud Always Free Ampere A1, or a
  Google Cloud `e2-micro`) is [`DEPLOY.md`](../DEPLOY.md) (Spanish).

With no reachable executor, set `JUDGE_WORKERS=0` so the gateway is a clean
producer: the REST/WebSocket API (auth, contests, problems, leaderboard) works
fully and submissions enqueue, but no verdict is produced until an executor
comes online.

## Gateway on Render

A committed Blueprint ([`render.yaml`](../render.yaml)) provisions the gateway.
Open `https://dashboard.render.com/blueprint/new?repo=<your-repo>` and fill the
secrets. Required environment (see
[`config.go`](../backend/services/api-gateway/internal/config/config.go) for the
full set):

| Variable | Value |
| --- | --- |
| `APP_ENV` | `production` (enables `Secure`/`SameSite=None` cookies; requires a strong `JWT_SECRET`) |
| `JWT_SECRET` | a strong random value (Render's `generateValue` does this) |
| `GATEWAY_HTTP_ADDR` | `:10000` (Render routes to 10000 by default) |
| `DATABASE_URL` | the Postgres connection string (Neon: the *direct*, non-pooler host, `?sslmode=require`) |
| `REDIS_ADDR` | a `host:port` or a `redis://` / `rediss://` URL (the Blueprint wires the Key Value's internal `connectionString`) |
| `EXECUTOR_ADDR` | the executor's address, or a placeholder + `JUDGE_WORKERS=0` |
| `EXECUTOR_TLS` | `true` when the executor sits behind a public TLS proxy (Caddy) |
| `EXECUTOR_AUTH_TOKEN` | shared bearer token; must equal the executor's `EXECUTOR_AUTH_TOKEN` |
| `FRONTEND_ORIGIN` | the exact frontend origin, e.g. `https://app.vercel.app` |

## Frontend on Vercel

Set `NEXT_PUBLIC_API_URL` to the gateway URL (e.g.
`https://aether-api-gateway.onrender.com`). The gateway's `FRONTEND_ORIGIN`
must equal the Vercel origin exactly (scheme + host, no trailing slash) — CORS
is an exact-match allow-list, and the same value gates WebSocket upgrades.

## Gotchas we hit (so you don't)

These are real failures encountered bringing this up, with the fix:

1. **Docker build context.** Both service Dockerfiles `COPY pkg/` and
   `COPY services/...`, so the build context **must be `backend/`**, not the
   service subfolder. On Render set *Docker Build Context Directory* = `backend`
   and *Dockerfile Path* = `backend/services/<svc>/Dockerfile` (the Blueprint's
   `dockerContext: ./backend` does this). A service-folder root yields
   `"/services/...": not found`.

2. **Region must match.** Render's short internal hostnames (`dpg-...`,
   `red-...`) only resolve on the private network **within the same region**. A
   gateway in a different region than its datastores fails with `lookup ... no
   such host`. Put the web service, Postgres, and Key Value all in one region.
   Postgres also offers a public *External* URL (with `sslmode=require`) that
   resolves anywhere; Key Value does **not**.

3. **Redis address format.** `REDIS_ADDR` accepts a bare `host:port`
   (plaintext, no auth) or a `redis://` / `rediss://` URL (credentials, TLS).
   The Render Key Value's *internal* URL needs no password; keep the instance
   in the gateway's region and `ipAllowList: []` so it is never public.

4. **Cross-site session cookie.** With the frontend and API on different sites
   (`*.vercel.app` ↔ `*.onrender.com`), a `SameSite=Lax` refresh cookie is never
   sent, so sessions silently drop after the access token expires. Production
   emits `SameSite=None; Secure` to fix this (ADR-0013); it requires HTTPS,
   which Render terminates.

## Security note

The production posture (TLS at the edge, security headers, `SameSite=None`
cookies, fail-closed rate limits, the sandbox isolation model, and the residual
risks) is documented in the [threat model](security/threat-model.md). For a
hostile-by-default production judge, run the executor under gVisor/Kata or a
microVM rather than the default Docker runtime.
