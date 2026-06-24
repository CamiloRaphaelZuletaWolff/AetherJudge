import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

import { apiFetch, ApiError, setAccessToken } from "./api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const echoSchema = z.object({ ok: z.boolean() });

describe("apiFetch auth retry", () => {
  beforeEach(() => {
    setAccessToken("stale-token");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    setAccessToken(null);
  });

  it("refreshes once on 401 and retries with the new token", async () => {
    const calls: Array<{ url: string; auth: string | null }> = [];

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const headers = new Headers(init?.headers);
        calls.push({ url, auth: headers.get("Authorization") });

        if (url.endsWith("/auth/refresh")) {
          return jsonResponse(200, { access_token: "fresh-token" });
        }
        // First protected call fails; the retry (fresh token) succeeds.
        if (headers.get("Authorization") === "Bearer stale-token") {
          return jsonResponse(401, { error: { code: "unauthorized", message: "expired" } });
        }
        return jsonResponse(200, { ok: true });
      }),
    );

    const result = await apiFetch("/api/v1/me", { schema: echoSchema, auth: true });

    expect(result.ok).toBe(true);
    const refreshCalls = calls.filter((c) => c.url.endsWith("/auth/refresh"));
    expect(refreshCalls).toHaveLength(1);
    expect(calls.at(-1)?.auth).toBe("Bearer fresh-token");
  });

  it("shares a single refresh across concurrent 401s", async () => {
    let refreshCount = 0;

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const headers = new Headers(init?.headers);

        if (url.endsWith("/auth/refresh")) {
          refreshCount += 1;
          // Slow refresh so both 401 handlers overlap.
          await new Promise((r) => setTimeout(r, 25));
          return jsonResponse(200, { access_token: "fresh-token" });
        }
        if (headers.get("Authorization") === "Bearer stale-token") {
          return jsonResponse(401, { error: { code: "unauthorized", message: "expired" } });
        }
        return jsonResponse(200, { ok: true });
      }),
    );

    const [a, b] = await Promise.all([
      apiFetch("/api/v1/a", { schema: echoSchema, auth: true }),
      apiFetch("/api/v1/b", { schema: echoSchema, auth: true }),
    ]);

    expect(a.ok).toBe(true);
    expect(b.ok).toBe(true);
    // The whole point: rotation-with-reuse-detection means a second
    // concurrent refresh would kill the session family.
    expect(refreshCount).toBe(1);
  });

  it("throws session_expired when refresh fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/auth/refresh")) {
          return jsonResponse(401, { error: { code: "unauthorized", message: "no cookie" } });
        }
        return jsonResponse(401, { error: { code: "unauthorized", message: "expired" } });
      }),
    );

    await expect(apiFetch("/api/v1/me", { schema: echoSchema, auth: true })).rejects.toMatchObject({
      code: "session_expired",
      status: 401,
    });
  });

  it("surfaces the backend error envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(409, { error: { code: "username_taken", message: "that username is taken" } }),
      ),
    );

    try {
      await apiFetch("/api/v1/auth/signup", { method: "POST", body: {}, schema: echoSchema });
      expect.unreachable("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).code).toBe("username_taken");
      expect((err as ApiError).status).toBe(409);
    }
  });
});
