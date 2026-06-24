# ADR-0007: JWT access tokens + rotated opaque refresh tokens, bcrypt, Redis rate limits

- Status: accepted
- Date: 2026-06-11
- Phase: 3

## Context

The gateway needs stateless request authentication (including on WebSocket
upgrades), long-lived sessions that survive access-token expiry, and
protection of the credential-guessing surface. Auth is the most
attacker-facing code in the system after the executor.

## Decision

**Access tokens** are HS256 JWTs (golang-jwt/v5), 15-minute TTL, carrying
`sub` (user id) and `username`. Verification enforces algorithm, issuer, and
expiry; all failures collapse into one opaque "invalid token" error so
responses never reveal *why* a token failed.

**Refresh tokens** are 256-bit random opaque values:

- Only the SHA-256 of a token is stored — a database leak yields nothing
  replayable.
- Every `/auth/refresh` **rotates**: the presented token is revoked (with a
  `replaced_by` link) and a new one is issued.
- Presenting an already-rotated token is treated as theft: the user's entire
  token family is revoked and the session dies everywhere.
- Delivery is an **httpOnly, SameSite=Lax cookie scoped to
  `/api/v1/auth`** (Secure in production) — JavaScript can never read it,
  and it rides only auth requests.

**Passwords** are bcrypt (cost 12), with inputs >72 bytes rejected rather
than silently truncated. Login compares against a dummy hash when the user
doesn't exist, flattening the timing difference between "unknown user" and
"wrong password".

**Rate limiting** is Redis fixed-window (`INCR` + `EXPIRE`): per-IP on auth
routes, per-user on submissions and ad-hoc runs. **Fail-closed** — if Redis
is down, the guarded routes refuse (503) rather than open up. Fixed windows
allow up to 2× burst at window boundaries; that's acceptable for abuse
protection and much simpler than sliding windows.

## Alternatives rejected

- **Argon2id** over bcrypt: stronger on paper, but x/crypto's argon2 needs
  hand-rolled parameter encoding/storage — more security-critical custom
  code than this threat model justifies.
- **Refresh tokens in localStorage**: readable by any XSS payload; the
  httpOnly cookie removes that entire class.
- **Long-lived JWTs as refresh tokens**: not revocable without a denylist,
  which reintroduces state anyway — opaque + hashed rows are simpler and
  strictly better here.

## Consequences

- CORS must run in credentials mode for exactly one origin
  (`FRONTEND_ORIGIN`); the wildcard origin is structurally impossible.
- WebSocket upgrades authenticate via `?access_token=` because browsers
  cannot set headers on WS handshakes; acceptable with 15-minute tokens and
  TLS in production, and noted in the threat model (Phase 8).
- Production refuses to boot with the dev JWT secret or one shorter than 32
  characters (config validation).
