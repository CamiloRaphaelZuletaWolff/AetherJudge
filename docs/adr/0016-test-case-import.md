# ADR-0016: Bulk test-case import from a file

- Status: accepted (2026-06-28)
- Phase: post-roadmap (content-authoring follow-on)
- Related: ADR-0015 (admin content authoring), ADR-0014 (RBAC)

## Context

Admins authored hidden test cases one row at a time (ADR-0015) — painful for
problems with many cases. This adds **file upload**: a `.txt`/`.md`/`.csv`/
`.json`/`.xlsx` file is turned into test cases in one shot. Test cases are the
graded answers, so this stays behind the existing **`problem.manage`**
permission (admins/moderators).

## Decision

**Parse on the server, but don't write — let the existing batch insert commit.**
A new endpoint `POST /api/v1/admin/test-cases/parse` (multipart `file`,
`problem.manage`) parses the upload into `{stdin, expected_output}` pairs and
returns them; it performs **no DB write**. The frontend drops the parsed cases
into the existing editable rows (`TestCaseEditor`) for review/edit, and the admin
saves through the already-tested, validated batch endpoint
`POST /admin/problems/{id}/test-cases`. So:

- Parsing is robust and multi-format **server-side** (Go), avoiding a heavy,
  audit-risky in-browser xlsx library.
- The user still gets a **preview + edit** step before anything is graded.
- The actual write reuses one validated path — no second insert/validation code.

**Formats** (each row/case → `{stdin, expected_output}`):
- `.csv` — two columns; a header row (`input/output/stdin/expected…`) is
  auto-detected and skipped (stdlib `encoding/csv`).
- `.xlsx` — first sheet, columns A/B, header auto-skipped (`xuri/excelize/v2`).
- `.json` — array of `{stdin|input, expected_output|output}` (stdlib).
- `.txt` / `.md` — **block format**: a `---` rule line splits a case's input
  from its expected output; a `===` rule line splits cases. Multi-line safe.

**Guardrails**: 8 MB upload cap (`http.MaxBytesReader`), ≤2000 cases per file,
each case run through the existing `validateTestCaseIO`, and unsupported
extensions / malformed content map to a 400 with a clear message. The parser
core (`parseTestCases`) is a pure function, unit-tested per format.

**Dependency**: `github.com/xuri/excelize/v2` is added to the api-gateway module
solely for `.xlsx` reading — well-maintained and the standard Go choice; it joins
the `govulncheck`/Trivy scan surface.

## Alternatives rejected

- **Client-side parsing** (incl. in-browser xlsx): the maintained npm `xlsx`
  (SheetJS) is unpublished/known-vulnerable and `exceljs` is heavy; server-side
  parsing is more robust and keeps the bundle lean.
- **Parse-and-insert in one endpoint**: loses the preview/edit step and would
  duplicate the validated insert path. Parse-only + existing batch is cleaner.

## Consequences

- Admins can load dozens/hundreds of cases from a spreadsheet or text file, then
  review and save — no more one-by-one entry.
- Covered by tests: per-format parser units (`.txt` block, `.csv`, `.json`, and
  an in-memory `.xlsx`), error cases, and an integration test (admin parse → 200;
  plain user → 403). No DB migration, no executor change.
