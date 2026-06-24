#!/usr/bin/env bash
#
# Runs govulncheck across every Go module and fails the build on any
# vulnerability that affects called code — EXCEPT a small, explicitly assessed
# allowlist of advisories that have no upstream fix and whose vulnerable code
# paths Arena never exercises. Each allowlisted ID is justified in
# docs/security/threat-model.md and docs/adr/0013-security-hardening.md.
#
# Requires Go >= 1.26.4 (older patch releases carry stdlib advisories that are
# fixed there; the go.mod `go` directive pins this).
set -uo pipefail

# Assessed, accepted advisories (no fix available; not reachable in our usage):
#   GO-2026-4887  Moby AuthZ plugin bypass on oversized bodies   (daemon AuthZ plugins — unused)
#   GO-2026-4883  Moby off-by-one in plugin privilege validation (docker plugins — unused)
ALLOW="GO-2026-4887 GO-2026-4883"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"

modules="pkg services/api-gateway services/executor"
overall=0

for m in $modules; do
  echo "== govulncheck: $m =="
  out="$(cd "$m" && govulncheck ./... 2>&1)"
  code=$?
  echo "$out"
  [ "$code" -eq 0 ] && continue

  # Non-zero exit means at least one advisory affects called code. Collect the
  # affected IDs from the "Symbol Results" section (everything before the
  # non-fatal "=== Informational ===" block, if present).
  called="$(printf '%s\n' "$out" | sed '/=== Informational ===/,$d' | grep -oE 'GO-[0-9]{4}-[0-9]+' | sort -u)"
  for id in $called; do
    if printf ' %s ' "$ALLOW" | grep -q " $id "; then
      echo "::notice::$m: accepted advisory $id (allowlisted — no upstream fix, not reachable)"
    else
      echo "::error::$m: unaccepted vulnerability $id"
      overall=1
    fi
  done
done

if [ "$overall" -ne 0 ]; then
  echo "FAIL: unaccepted vulnerabilities found (see above)"
  exit 1
fi
echo "OK: no unaccepted vulnerabilities"
