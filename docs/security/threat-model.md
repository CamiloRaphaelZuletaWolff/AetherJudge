# Arena — Threat model

This document records the security posture of Arena: the assets worth
protecting, the trust boundaries they sit behind, the threats against each
boundary, the controls in place today, and the residual risks we accept. It is
written against the system as built through Phase 8.

The framing is STRIDE (Spoofing, Tampering, Repudiation, Information
disclosure, Denial of service, Elevation of privilege), applied per trust
boundary rather than per component.

## Assets

| Asset | Why it matters |
| --- | --- |
| **The host running the executor** | Arena compiles and runs *attacker-supplied code*. Escaping the sandbox onto the host is the worst-case outcome. |
| User credentials & sessions | Password hashes, JWT signing key, refresh tokens. |
| Contest integrity | Hidden test cases, submissions, and the leaderboard must not be forgeable or leakable ahead of time. |
| Availability of judging | A shared, finite sandbox pool is a natural DoS target. |
| PostgreSQL data | The single source of truth (ADR-0004). |

## Trust boundaries

```
            (1) Internet / browser
                     │  REST + WebSocket (TLS at the edge)
                     ▼
        ┌─────────────────────────┐
        │   api-gateway (public)  │
        └─────────────────────────┘
            │ gRPC (2)        │ SQL / Redis (3)
            ▼                 ▼
   ┌──────────────┐   ┌──────────────────────┐
   │  executor    │   │ PostgreSQL · Redis   │
   └──────────────┘   └──────────────────────┘
            │ Docker API (4)  — THE CRITICAL BOUNDARY
            ▼
   ┌──────────────────────────────────────────┐
   │ ephemeral sandbox container (hostile code)│
   └──────────────────────────────────────────┘
```

1. **Browser → gateway** — fully untrusted clients.
2. **Gateway → executor** — internal gRPC; trusted network, but the *payload*
   (user code) is hostile and crosses here.
3. **Gateway → datastores** — internal; PostgreSQL is the source of truth.
4. **Executor → sandbox** — the executor is trusted; everything *inside* the
   sandbox is the adversary. This is the boundary that matters most.

---

## Boundary 4 — Sandbox (the crown jewel)

The executor runs untrusted C++, Python, and Go. Assume the code inside a
sandbox is fully adversarial and will attempt to (a) reach the network, (b)
read or write the host filesystem, (c) consume unbounded CPU/memory/PIDs, (d)
escalate privileges, and (e) break out of the container.

**Controls (ADR-0005, ADR-0006), enforced on every container in
[`sandbox/engine.go`](../../backend/services/executor/internal/sandbox/engine.go)):**

| Threat | Control |
| --- | --- |
| Network exfiltration / C2 | `NetworkDisabled` + `NetworkMode: "none"` — no interface but loopback. |
| Host filesystem tampering / disclosure | `ReadonlyRootfs: true`; the only writable mount is a size-capped `tmpfs` at `/box`. |
| Privilege escalation | `CapDrop: ["ALL"]`, `SecurityOpt: ["no-new-privileges"]`, non-root user (uid 10001). |
| Kernel attack surface (syscalls) | Docker's **default seccomp profile remains active** — `SecurityOpt` is *not* set to `seccomp=unconfined`, so the ~40 dangerous syscalls Docker blocks by default stay blocked. (See "Seccomp decision" below.) |
| CPU / memory / fork-bomb DoS | Per-phase `Memory` (+ `MemorySwap` equal to it → swap disabled), `NanoCPUs`, and `PidsLimit` quotas, applied at create and tightened via `ContainerUpdate` before the run phase. |
| Wall-clock DoS | The container is `sleep`-driven and killed on a deadline; every phase has a timeout. |
| Container leak / resource exhaustion over time | Containers are labelled `arena.sandbox` and **force-removed on every exit path**; an integration test asserts zero leak. |
| Compile-bomb / output flooding | Generous-but-bounded compile limits, then a strict run envelope; output is size-capped (`EXECUTOR_OUTPUT_LIMIT_KB`). |

**Pipeline shape (ADR-0006):** exec-driven — no bind mounts, no `docker cp`.
Source is written *into* the container over the exec stream, so the host
filesystem is never exposed to the build.

**Seccomp decision (Phase 8):** we deliberately rely on Docker's *default*
seccomp profile rather than shipping a custom one. The default already blocks
the syscalls used in most container escapes, while a hand-rolled allow-list
risks silently breaking a compiler or language runtime (and would need
re-measuring against the pre-warmed caches). Combined with `CapDrop: ALL` and
`no-new-privileges`, the marginal value of a custom profile is low and the
breakage risk is real. A tighter per-language profile is recorded as future
work, not a Phase 8 deliverable.

**Residual risk:** a kernel 0-day reachable through the default-seccomp syscall
surface could still permit escape. We do not run gVisor/Kata or a per-pod
microVM. Mitigations: the executor pod is itself unprivileged (only its DinD
sidecar is privileged, ADR-0009), runs no secrets, and has no network path to
the datastores — a successful escape lands in a throwaway judging pod, not the
data tier. Defense-in-depth via gVisor is the recommended next step for a
hostile-by-default production deployment.

---

## Boundary 1 — Browser → gateway

**Spoofing / Elevation (authn & authz).** Passwords are bcrypt-hashed
(cost 12). Access is a short-lived (15 min) HS256 JWT; the refresh token is
rotated on every use, stored **hashed**, and a detected reuse revokes the whole
token family (ADR-0007). The refresh token rides an `HttpOnly`, `Secure`
(production) cookie with `SameSite=None` in production / `Lax` in dev. Every
mutating and private route is behind `requireAuth`.

**Tampering / Information disclosure (transport & CORS).** TLS is terminated at
the edge (Render/ingress). CORS is an **exact-match allow-list of a single
configured origin** with credentials — never a wildcard
([`middleware.go`](../../backend/services/api-gateway/internal/api/middleware.go)).
The same origin gates WebSocket upgrades. Response **security headers** are set
on every reply: `Content-Security-Policy: default-src 'none'; frame-ancestors
'none'` (safe because the gateway serves only JSON/WS, never HTML),
`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy:
no-referrer`, and HSTS in production.

**Tampering (input validation).** JSON bodies are decoded with a hard
`MaxBytesReader` size cap and `DisallowUnknownFields`; a single-object check
rejects trailing garbage ([`respond.go`](../../backend/services/api-gateway/internal/api/respond.go)).
Usernames, emails, passwords, language codes, and stdin are validated before
use. All SQL goes through parameterized queries (pgx) — no string-built SQL.

**Denial of service.** Fixed-window rate limits (Redis) guard the abuse
surfaces: auth routes by IP (credential-guessing), submit/run by user (each
costs a sandbox). The limiter is **fail-closed** — if Redis is unreachable, the
guarded routes return 503 rather than opening up. The submission queue applies
backpressure (HTTP 503 once the stream depth exceeds a limit). HTTP
`ReadHeaderTimeout`/`ReadTimeout`/`IdleTimeout` are set to bound slowloris-style
holds.

**Repudiation.** Every request is logged (structured `slog`) with method, path,
status, duration, and — when tracing is on — a `trace_id` correlating the HTTP
request through the queue, the gRPC call, and each sandbox phase.

**Residual risk — rate-limit keying behind a proxy.** `byIP` reads
`r.RemoteAddr`. Behind a reverse proxy (e.g. Render) that is the *proxy's* IP,
so per-IP auth limits degrade toward a shared global bucket. We do **not** trust
`X-Forwarded-For` today, because naive XFF trust is itself spoofable. The
mitigation path (a configurable trusted-proxy hop count) is recorded as future
work; per-user limits on the expensive routes are unaffected.

---

## Boundary 2 — Gateway → executor

Internal gRPC on a trusted network. The gateway **clamps** every resource
request (timeouts, memory) to configured maxima before calling `Execute`, and a
semaphore bounds executor concurrency, so a compromised or buggy gateway cannot
ask the executor for unbounded resources. The hostile payload (user code)
crosses here but is only ever handled inside a sandbox (Boundary 4). gRPC is not
currently mTLS-secured — acceptable while both services share a private network;
mTLS is the hardening step if that assumption ever breaks.

## Boundary 3 — Gateway → datastores

PostgreSQL is the only source of truth; Redis holds only rebuildable state
(queue, pub/sub, leaderboard cache, rate-limit counters) — losing Redis cannot
lose data (ADR-0004, ADR-0011, ADR-0012). Credentials come from the environment
only. Connection strings and secrets are never logged. The judging path is
idempotent, so at-least-once queue redelivery cannot double-score.

---

## Secrets

- `JWT_SECRET` is required to be strong (≥ 32 chars, not the dev/placeholder
  value) in production — startup *fails* otherwise
  ([`config.go`](../../backend/services/api-gateway/internal/config/config.go)).
- All configuration is environment-only; nothing secret is committed. The repo
  ships only `*.example` templates with placeholders, and a CI **gitleaks** scan
  guards against accidental secret commits.
- The git history was scrubbed of internal working docs before publication.

## Supply chain

- Go module checksums (`go.sum`) and a pinned tool set; `govulncheck` runs in CI
  against the known-vulnerability database.
- Frontend dependencies are pinned via `pnpm-lock.yaml`; native build scripts
  are explicitly allow-listed; `pnpm audit` runs in CI.
- Built container images are scanned with **Trivy**; Dependabot keeps Go
  modules, npm packages, GitHub Actions, and Dockerfiles current.

## What is explicitly out of scope

- WAF / DDoS protection at the edge (delegated to the hosting platform).
- gVisor/Kata/microVM sandbox isolation (documented as the next hardening step).
- mTLS between gateway and executor (private-network assumption).
- Multi-tenant secret management (KMS/Vault) — single-tenant portfolio scope.

## Summary of Phase 8 changes

- Added defensive response security headers (CSP/HSTS/nosniff/frame/referrer).
- Fixed the refresh cookie for cross-site production (`SameSite=None`).
- Documented the sandbox seccomp decision and the full control set (this doc).
- Added CI security gates: `govulncheck`, `pnpm audit`, Trivy image scan,
  gitleaks secret scan, and Dependabot.
- Added `SECURITY.md` (disclosure policy) and ADR-0013.
