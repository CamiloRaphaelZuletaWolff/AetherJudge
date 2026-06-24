# ADR-0013: Security hardening posture

- Status: accepted (2026-06-18)
- Phase: 8
- Related: ADR-0005/0006 (sandbox isolation & pipeline), ADR-0007 (auth),
  ADR-0004 (PG truth / Redis rebuildable), ADR-0009 (deploy shapes)

## Context

Arena's defining risk is running attacker-supplied code, and Phase 8 is the
"make the security posture explicit and close the gaps" phase. Much of the
hardening was already built in earlier phases (sandbox isolation, fail-closed
rate limiting, bcrypt + rotated refresh tokens, parameterized SQL, body-size
caps). This ADR records the decisions made when auditing the whole and the
deltas added in Phase 8. The full analysis is in
[docs/security/threat-model.md](../security/threat-model.md).

## Decisions

1. **Response security headers on the gateway.** The gateway serves only JSON
   and WebSocket upgrades — never HTML — so it sets the tightest possible
   `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, plus
   `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
   `Referrer-Policy: no-referrer`, and HSTS in production only (it is
   meaningless over local HTTP and would wrongly pin a dev box). Implemented as
   `withSecurityHeaders` wrapping the CORS layer.

2. **Refresh cookie `SameSite=None` in production.** The production deployment
   serves the SPA from a different site than the API (e.g. a Vercel frontend
   against a Render gateway), so a `SameSite=Lax` cookie would never be sent and
   silent refresh would break. Production now emits `SameSite=None` (paired with
   the already-set `Secure`, which `None` requires); local same-origin dev keeps
   `Lax`, since browsers reject `None` without `Secure` over HTTP.

3. **Rely on Docker's default seccomp profile; do not ship a custom one.**
   `CapDrop: ALL` + `no-new-privileges` + non-root + the default seccomp
   allow-list already block the syscall surface used in common escapes. A
   hand-rolled per-language profile risks silently breaking a compiler/runtime
   and would need re-measuring against the pre-warmed caches; the marginal gain
   does not justify the breakage risk at this scope. A tighter profile (or
   gVisor) is recorded as future work.

4. **Do not trust `X-Forwarded-For` (yet).** Per-IP auth rate limits key on
   `r.RemoteAddr`, which degrades behind a reverse proxy. Naive XFF trust is
   spoofable, so we accept the degradation and document a configurable
   trusted-proxy-hop mitigation as future work rather than ship a spoofable one.
   Per-user limits on the expensive (sandbox-consuming) routes are unaffected.

5. **Supply-chain gates in CI.** Add `govulncheck` (Go), `pnpm audit`
   (frontend), Trivy (container images), and gitleaks (secret scanning) as CI
   jobs, plus Dependabot for Go modules, npm, GitHub Actions, and Dockerfiles.
   Vulnerability gates fail on HIGH/CRITICAL to stay actionable rather than
   noisy.

## Consequences

- The security posture is now documented and testable; the security-headers
  middleware has a unit test, and the cookie change is covered by the existing
  refresh-flow integration test.
- CI gains security signal on every push, and Dependabot keeps dependencies
  current — at the cost of occasional triage when a scanner flags a transitive
  advisory.
- Residual risks (kernel-0-day sandbox escape, proxy rate-limit keying, no
  gateway↔executor mTLS) are explicitly accepted and documented with their
  mitigation paths, rather than left implicit.
