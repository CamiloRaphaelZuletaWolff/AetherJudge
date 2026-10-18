import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useServerWake } from "./use-server-wake";

const URL = "https://api.example.test/healthz";
const opts = { url: URL, slowAfterMs: 2_000, retryMs: 3_000, giveUpAfterMs: 20_000 };

// A fetch whose responses the test resolves by hand.
function controllableFetch() {
  const pending: Array<{ resolve: (r: Response) => void; reject: (e: unknown) => void }> = [];
  const fn = vi.fn(
    (_input: RequestInfo | URL, init?: RequestInit) =>
      new Promise<Response>((resolve, reject) => {
        pending.push({ resolve, reject });
        init?.signal?.addEventListener("abort", () =>
          reject(new DOMException("aborted", "AbortError")),
        );
      }),
  );
  return { fn, pending };
}

describe("useServerWake", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("stays silent when the server answers within the grace period", async () => {
    const f = controllableFetch();
    vi.stubGlobal("fetch", f.fn);
    const { result } = renderHook(() => useServerWake(opts));

    expect(result.current).toBe("checking");
    expect(f.fn).toHaveBeenCalledWith(URL, expect.objectContaining({ cache: "no-store" }));

    await act(async () => f.pending[0]!.resolve(new Response("ok", { status: 200 })));
    expect(result.current).toBe("ready");

    // The slow timer must not resurrect the banner afterwards.
    act(() => vi.advanceTimersByTime(30_000));
    expect(result.current).toBe("ready");
  });

  it("reports waking after 2 s without an answer, then ready when healthy", async () => {
    const f = controllableFetch();
    vi.stubGlobal("fetch", f.fn);
    const { result } = renderHook(() => useServerWake(opts));

    act(() => vi.advanceTimersByTime(1_999));
    expect(result.current).toBe("checking");
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe("waking");

    // Render's proxy answers 502 while the instance boots: retried.
    await act(async () => f.pending[0]!.resolve(new Response("", { status: 502 })));
    expect(result.current).toBe("waking");
    await act(async () => vi.advanceTimersByTime(3_000));
    expect(f.fn).toHaveBeenCalledTimes(2);

    await act(async () => f.pending[1]!.resolve(new Response("ok", { status: 200 })));
    expect(result.current).toBe("ready");
  });

  it("retries network errors and escalates to unreachable, still recovering later", async () => {
    const f = controllableFetch();
    vi.stubGlobal("fetch", f.fn);
    const { result } = renderHook(() => useServerWake(opts));

    await act(async () => f.pending[0]!.reject(new TypeError("Failed to fetch")));
    await act(async () => vi.advanceTimersByTime(20_000));
    expect(result.current).toBe("unreachable");
    expect(f.fn.mock.calls.length).toBeGreaterThan(1);

    const last = f.pending[f.pending.length - 1]!;
    await act(async () => last.resolve(new Response("ok", { status: 200 })));
    expect(result.current).toBe("ready");
  });

  it("aborts the in-flight ping on unmount", () => {
    const f = controllableFetch();
    vi.stubGlobal("fetch", f.fn);
    const { unmount } = renderHook(() => useServerWake(opts));

    const signal = f.fn.mock.calls[0]![1]!.signal!;
    unmount();
    expect(signal.aborted).toBe(true);
  });
});
