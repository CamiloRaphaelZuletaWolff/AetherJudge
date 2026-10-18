"use client";

// Cold-start detection for the API. On Render's free plan the gateway sleeps
// after 15 idle minutes and the first request takes ~1 minute while it boots
// — without a hint the demo just looks broken. This hook pings /healthz once
// per mount and reports:
//
//   checking     ping in flight, still within the grace period (show nothing)
//   waking       no healthy answer after `slowAfterMs` — the server is asleep
//   unreachable  still nothing after `giveUpAfterMs` (keeps retrying anyway)
//   ready        /healthz answered 2xx
//
// Render holds the first request open while the instance boots, so a single
// long-lived fetch usually resolves on its own; network errors and 5xx (the
// proxy's 502 while booting) are retried every `retryMs`. The ping is a CORS
// request, so only the gateway itself (which sends Access-Control-Allow-Origin
// for FRONTEND_ORIGIN) can mark the server ready — a proxy/interstitial page
// without that header makes fetch reject and counts as "still asleep".
import { useEffect, useState } from "react";

import { API_URL } from "@/lib/api";

export type ServerWakeState = "checking" | "waking" | "unreachable" | "ready";

export interface ServerWakeOptions {
  url?: string;
  slowAfterMs?: number;
  retryMs?: number;
  attemptTimeoutMs?: number;
  giveUpAfterMs?: number;
}

export const WAKE_SLOW_AFTER_MS = 2_000;

export function useServerWake({
  url = `${API_URL}/healthz`,
  slowAfterMs = WAKE_SLOW_AFTER_MS,
  retryMs = 3_000,
  attemptTimeoutMs = 75_000,
  giveUpAfterMs = 150_000,
}: ServerWakeOptions = {}): ServerWakeState {
  const [state, setState] = useState<ServerWakeState>("checking");

  useEffect(() => {
    let done = false;
    let controller: AbortController | null = null;
    const timers = new Set<ReturnType<typeof setTimeout>>();
    const later = (fn: () => void, ms: number) => {
      const id = setTimeout(() => {
        timers.delete(id);
        fn();
      }, ms);
      timers.add(id);
    };
    const finish = () => {
      done = true;
      timers.forEach(clearTimeout);
      timers.clear();
    };

    later(() => setState((s) => (s === "checking" ? "waking" : s)), slowAfterMs);
    later(() => setState((s) => (s === "ready" ? s : "unreachable")), giveUpAfterMs);

    const attempt = async () => {
      controller = new AbortController();
      const abort = controller.abort.bind(controller);
      const attemptTimer = setTimeout(abort, attemptTimeoutMs);
      try {
        const res = await fetch(url, {
          cache: "no-store",
          credentials: "omit",
          signal: controller.signal,
        });
        if (res.ok) {
          if (!done) {
            finish();
            setState("ready");
          }
          return;
        }
      } catch {
        // Asleep, booting, or aborted — fall through to the retry.
      } finally {
        clearTimeout(attemptTimer);
      }
      if (!done) later(() => void attempt(), retryMs);
    };
    void attempt();

    return () => {
      finish();
      controller?.abort();
    };
  }, [url, slowAfterMs, retryMs, attemptTimeoutMs, giveUpAfterMs]);

  return state;
}
