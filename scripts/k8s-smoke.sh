#!/usr/bin/env bash
# Smoke test for the in-cluster stack: signup → submit → judged `accepted`
# → leaderboard, all through the gateway NodePort. Deliberately jq-free so
# it runs in Git Bash on Windows and on CI runners alike.
set -euo pipefail

BASE_URL="${ARENA_URL:-http://localhost:8091}"
USER="smoke$(date +%s)"

echo "==> waiting for readiness at ${BASE_URL}"
for i in $(seq 1 60); do
    if curl -fsS "${BASE_URL}/readyz" >/dev/null 2>&1; then
        break
    fi
    [ "$i" -eq 60 ] && { echo "gateway never became ready"; exit 1; }
    sleep 2
done

echo "==> signup as ${USER}"
signup=$(curl -fsS -X POST "${BASE_URL}/api/v1/auth/signup" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${USER}\",\"email\":\"${USER}@example.com\",\"password\":\"password123\"}")
token=$(printf '%s' "$signup" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
[ -n "$token" ] || { echo "no access token in: $signup"; exit 1; }

echo "==> find the seeded contest"
contests=$(curl -fsS "${BASE_URL}/api/v1/contests?filter=active")
contest_id=$(printf '%s' "$contests" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -n 1)
[ -n "$contest_id" ] || { echo "no active contest in: $contests"; exit 1; }
echo "    contest ${contest_id}"

echo "==> submit a correct python solution to problem 1"
submission=$(curl -fsS -X POST \
    "${BASE_URL}/api/v1/contests/${contest_id}/problems/1/submissions" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d '{"language":"python","code":"a, b = map(int, input().split())\nprint(a + b)"}')
submission_id=$(printf '%s' "$submission" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
[ -n "$submission_id" ] || { echo "no submission id in: $submission"; exit 1; }
echo "    submission ${submission_id}"

echo "==> wait for the verdict (sandbox judging inside the cluster)"
verdict=""
for i in $(seq 1 90); do
    status=$(curl -fsS "${BASE_URL}/api/v1/submissions/${submission_id}" \
        -H "Authorization: Bearer ${token}")
    verdict=$(printf '%s' "$status" | sed -n 's/.*"verdict":"\([^"]*\)".*/\1/p')
    if printf '%s' "$status" | grep -q '"status":"done"'; then
        break
    fi
    sleep 2
done

if [ "$verdict" != "accepted" ]; then
    echo "FAIL: verdict='${verdict}' (wanted accepted)"
    exit 1
fi
echo "    verdict: accepted"

echo "==> leaderboard shows the new user"
# Scoring commits just after the verdict becomes readable; retry briefly.
board=""
for i in $(seq 1 15); do
    board=$(curl -fsS "${BASE_URL}/api/v1/contests/${contest_id}/leaderboard")
    if printf '%s' "$board" | grep -q "\"username\":\"${USER}\""; then
        break
    fi
    [ "$i" -eq 15 ] && { echo "FAIL: ${USER} missing from leaderboard: $board"; exit 1; }
    sleep 2
done

echo "SMOKE OK: submission judged accepted inside the cluster"
