// k6 load test: drives the real submission path (signup → submit → poll
// verdict) at a configurable concurrency and reports judge latency, so the
// queue + executor fan-out can be measured under load (Phase 7 / ADR-0011).
//
//   k6 run loadtest/submit.js                 # defaults: ramp to 10 VUs
//   ARENA_URL=http://localhost:8091 VUS=25 DURATION=2m k6 run loadtest/submit.js
//   k6 run -o experimental-prometheus-rw loadtest/submit.js   # → Grafana
//
// The gateway must run with generous rate limits for a benchmark (this test
// measures judging throughput, not rate-limiting):
//   RATE_LIMIT_AUTH_PER_MIN=100000 RATE_LIMIT_SUBMIT_PER_MIN=100000
import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";

const BASE = __ENV.ARENA_URL || "http://localhost:8080";
const VUS = __ENV.VUS ? Number(__ENV.VUS) : 10;
const DURATION = __ENV.DURATION || "1m";

const judgeLatency = new Trend("judge_latency_ms", true);
const acceptedRate = new Rate("verdict_accepted");

export const options = {
  scenarios: {
    submissions: {
      executor: "ramping-vus",
      startVUs: 1,
      stages: [
        { duration: "20s", target: VUS },
        { duration: DURATION, target: VUS },
        { duration: "10s", target: 0 },
      ],
    },
  },
  thresholds: {
    // Submitting must stay healthy; judging is seconds-scale on this hardware.
    http_req_failed: ["rate<0.05"],
    judge_latency_ms: ["p(95)<90000"],
    verdict_accepted: ["rate>0.95"],
  },
};

const JSON_HEADERS = { "Content-Type": "application/json" };

function authHeaders(token) {
  return { headers: { ...JSON_HEADERS, Authorization: `Bearer ${token}` } };
}

function newUser(tag) {
  const name = `load_${tag}_${Date.now()}`;
  const res = http.post(
    `${BASE}/api/v1/auth/signup`,
    JSON.stringify({ username: name, email: `${name}@example.com`, password: "password123" }),
    { headers: JSON_HEADERS },
  );
  // Signup returns 201 Created.
  return res.status === 201 ? res.json("access_token") : null;
}

// setup resolves the active contest once, with a throwaway account.
export function setup() {
  const token = newUser("setup");
  if (!token) throw new Error("setup signup failed (raise RATE_LIMIT_AUTH_PER_MIN?)");
  const res = http.get(`${BASE}/api/v1/contests?filter=active`);
  // The list endpoint returns {contests:[...]}; fall back to a bare array.
  const contestId = res.json("contests.0.id") || res.json("0.id");
  if (!contestId) throw new Error(`no active contest in: ${res.body}`);
  return { contestId };
}

export default function (data) {
  const token = newUser(`${__VU}_${__ITER}`);
  if (!check(token, { "signup ok": (t) => !!t })) return;

  const code = "a, b = map(int, input().split())\nprint(a + b)";
  const submit = http.post(
    `${BASE}/api/v1/contests/${data.contestId}/problems/1/submissions`,
    JSON.stringify({ language: "python", code }),
    authHeaders(token),
  );
  if (!check(submit, { "submit accepted (202)": (r) => r.status === 202 })) return;

  const subId = submit.json("id");
  const start = Date.now();

  let verdict = "";
  for (let i = 0; i < 90; i++) {
    const s = http.get(`${BASE}/api/v1/submissions/${subId}`, authHeaders(token));
    if (s.json("status") === "done") {
      verdict = s.json("verdict");
      break;
    }
    sleep(1);
  }

  judgeLatency.add(Date.now() - start);
  acceptedRate.add(verdict === "accepted");
  check(verdict, { "verdict accepted": (v) => v === "accepted" });
}
