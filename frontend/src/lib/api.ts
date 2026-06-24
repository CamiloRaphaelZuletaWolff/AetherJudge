// Typed API client. Owns the in-memory access token and the single-flight
// refresh: on a 401, exactly one /auth/refresh runs no matter how many
// requests hit it concurrently — parallel refreshes would trip the
// backend's rotation reuse-detection and revoke the whole session family
// (see docs/adr/0007-auth-design.md).
import type { z } from "zod";

import { refreshResponseSchema } from "@/lib/schemas";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export function wsURL(path: string): string {
  return API_URL.replace(/^http/, "ws") + path;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// The access token lives in module memory only — never in storage — so XSS
// cannot exfiltrate it. Reloads recover the session through the httpOnly
// refresh cookie.
let accessToken: string | null = null;
let onSessionExpired: (() => void) | null = null;

export function setAccessToken(token: string | null): void {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

// registerSessionExpiredHandler lets the auth store react (log out) when a
// refresh attempt fails — without the api module importing the store.
export function registerSessionExpiredHandler(handler: () => void): void {
  onSessionExpired = handler;
}

let refreshInFlight: Promise<boolean> | null = null;

// refreshSession exchanges the refresh cookie for a new access token.
// Single-flight: concurrent callers share one request. Returns whether the
// session is now valid.
export function refreshSession(): Promise<boolean> {
  refreshInFlight ??= (async () => {
    try {
      const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) {
        setAccessToken(null);
        return false;
      }
      const body = refreshResponseSchema.parse(await res.json());
      setAccessToken(body.access_token);
      return true;
    } catch {
      setAccessToken(null);
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

// ensureFreshToken returns a usable access token, refreshing if none is
// held (used by the WebSocket hook before dialing).
export async function ensureFreshToken(): Promise<string | null> {
  if (accessToken) return accessToken;
  await refreshSession();
  return accessToken;
}

interface RequestOptions<T> {
  method?: "GET" | "POST";
  body?: unknown;
  schema?: z.ZodType<T>;
  // auth attaches the bearer token and enables the 401→refresh→retry path.
  auth?: boolean;
}

export async function apiFetch<T = undefined>(
  path: string,
  { method = "GET", body, schema, auth = false }: RequestOptions<T> = {},
): Promise<T> {
  const doFetch = async (): Promise<Response> => {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (auth && accessToken) headers["Authorization"] = `Bearer ${accessToken}`;

    return fetch(`${API_URL}${path}`, {
      method,
      headers,
      credentials: "include",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  };

  let res = await doFetch();

  if (res.status === 401 && auth) {
    const recovered = await refreshSession();
    if (!recovered) {
      onSessionExpired?.();
      throw new ApiError(401, "session_expired", "your session has expired");
    }
    res = await doFetch();
  }

  if (!res.ok) {
    let code = "unknown";
    let message = `request failed with status ${res.status}`;
    try {
      const envelope = (await res.json()) as { error?: { code?: string; message?: string } };
      code = envelope.error?.code ?? code;
      message = envelope.error?.message ?? message;
    } catch {
      // Non-JSON error body; keep the generic message.
    }
    throw new ApiError(res.status, code, message);
  }

  if (res.status === 204 || schema === undefined) {
    return undefined as T;
  }
  return schema.parse(await res.json());
}
