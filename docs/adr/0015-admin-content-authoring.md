# ADR-0015: Admin contest & problem authoring

- Status: accepted (2026-06-28)
- Phase: post-roadmap (RBAC follow-on)
- Related: ADR-0014 (RBAC), ADR-0007 (auth), ADR-0004 (PG truth / Redis rebuildable)

## Context

RBAC (ADR-0014) shipped the `contest.create` / `contest.edit` / `problem.manage` permissions but
nothing exercised them: contests, problems, and test cases could only be created by editing
`cmd/seed` and re-running it. This change makes those permissions real — an admin can create a
scheduled contest, add problems (markdown statement + limits), and attach hidden test cases from an
in-app authoring UI. The `contests` / `problems` / `test_cases` tables and the
`CreateContest`/`CreateProblem`/`CreateTestCase` store methods already existed (the seed uses them),
so authored content is judgeable immediately through the existing submit→judge path.

## Decision

**Endpoints** (under `/api/v1/admin/`, each `requireAuth` + `requirePermission`):
`POST /contests` (`contest.create`), `PATCH /contests/{id}` (`contest.edit`),
`GET|POST /contests/{id}/problems` (`problem.manage`),
`GET|POST /problems/{problemId}/test-cases` (`problem.manage`). Authorization is enforced
server-side exactly as for the rest of the admin surface; the UI gating is convenience only.

- **Strict create, separate from the seed's upsert.** A new `InsertContest` does a plain `INSERT`
  and maps a unique-violation on the slug to `ErrSlugTaken` → HTTP 409, so an admin can't silently
  overwrite an existing contest. The pre-existing `CreateContest` (which upserts on slug) stays, but
  is reserved for idempotent **seeding**. Same spirit for problems/test cases: ordinals are
  **auto-assigned** server-side (`NextProblemOrd` / `NextTestCaseOrd` = `MAX(ord)+1`), so the
  existing conflict-free `CreateProblem`/`CreateTestCase` are reused and the client never picks an
  ordinal.
- **Slug**: auto-derived from the title (`slugify`, mirrored on the frontend) when the admin leaves
  it blank, then validated against the table's CHECK regex; immutable on edit (it keys joins/URLs).
- **Test cases stay all-hidden — no schema change.** There is no "sample/visible" flag; worked
  examples live in the problem statement markdown (matching how seeded problems already work). This
  keeps the judge and the schema untouched.
- **Validation mirrors the DB CHECK constraints** (slug, title 1–200, time 100–10000 ms, memory
  16–512 MB, end-after-start) in `internal/api/validate.go`, with the constraints as the final
  backstop.
- **Authoring UI is admin-only** (lives under the `admin.access`-gated `/admin` surface) even though
  the underlying content permissions are granted to **moderator+** in the RBAC model. The API
  remains the boundary, so a moderator retains the capability programmatically; only the UI is
  scoped to admins for now.

## Alternatives rejected

- **Reusing the seed's upsert `CreateContest` for the API**: silently overwrites a contest on a slug
  clash — wrong for an interactive "create". Hence the strict `InsertContest`.
- **Client-supplied ordinals**: invites races and off-by-one bugs; server-assigned `MAX(ord)+1` is
  simpler and conflict-free.
- **A `test_cases.is_sample` column**: more schema and judge surface than warranted; examples in the
  statement already serve that role.

## Consequences

- Contests are now fully self-service for admins — no redeploy, no seed edits. New content is
  immediately playable and judged by the existing pipeline.
- Covered by tests: validator units, and an integration test that authors a contest → problem →
  test cases over the API and asserts a submission to the authored problem judges `accepted`, plus
  the 403 (non-admin), 409 (duplicate slug), and 400 (bad limits) guardrails.
- Deferred (seam intact): delete/reorder of contests/problems/cases, rejudge, and a sample-case
  flag. No DB migration and no executor change were required.
