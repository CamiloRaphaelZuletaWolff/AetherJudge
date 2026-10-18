import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SCORE_REVEAL_MS, SCORE_SETTLE_MS, useScoreboard } from "./use-scoreboard";
import type { LeaderboardRow } from "@/lib/schemas";

const initial: LeaderboardRow[] = [
  { username: "alice", rank: 1, solved: 2, penalty_s: 100 },
  { username: "bob", rank: 2, solved: 1, penalty_s: 200 },
  { username: "carol", rank: 3, solved: 1, penalty_s: 300 },
];
const promoted: LeaderboardRow[] = [
  { username: "carol", rank: 1, solved: 3, penalty_s: 400 },
  { ...initial[0]!, rank: 2 },
  { ...initial[1]!, rank: 3 },
];
const fullMove = SCORE_REVEAL_MS + SCORE_SETTLE_MS;

describe("ICPC scoreboard presentation", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("does not celebrate the initial snapshot; reveals a solve before moving the same competitor", () => {
    const { result, rerender } = renderHook(({ entries }) => useScoreboard(entries, true, false), {
      initialProps: { entries: initial },
    });
    expect(result.current.changes).toEqual([]);
    expect(result.current.announcement).toBe("");
    rerender({ entries: promoted });
    expect(result.current.rows.map((r) => r.username)).toEqual(["alice", "bob", "carol"]);
    expect(result.current.rows[2]).toMatchObject({ rank: 3, solved: 3 });
    expect(result.current.changes[0]).toMatchObject({ username: "carol", gained: 2 });
    act(() => vi.advanceTimersByTime(SCORE_REVEAL_MS));
    expect(result.current.rows).toEqual(promoted);
    act(() => vi.advanceTimersByTime(SCORE_SETTLE_MS));
    expect(result.current.changes).toEqual([]);
    expect(result.current.announcement).toContain("carol climbs 2 places to #1");
  });

  it("coalesces updates during a climb and settles on the latest authoritative order", () => {
    const { result, rerender } = renderHook(({ entries }) => useScoreboard(entries, true, false), {
      initialProps: { entries: initial },
    });
    rerender({ entries: promoted });
    rerender({ entries: [] });
    const latest = [
      { ...initial[1]!, rank: 1, solved: 5 },
      { ...promoted[0]!, rank: 2 },
    ];
    rerender({ entries: latest });
    act(() => vi.advanceTimersByTime(fullMove * 2));
    expect(result.current.rows).toEqual(latest);
    expect(result.current.behind).toBe(false);
    expect(result.current.changes).toEqual([]);
  });

  it("pauses during a reveal and resumes using the latest data", () => {
    const { result, rerender } = renderHook(
      ({ entries, paused }) => useScoreboard(entries, true, paused),
      { initialProps: { entries: initial, paused: false } },
    );
    rerender({ entries: promoted, paused: false });
    rerender({ entries: promoted, paused: true });
    act(() => vi.advanceTimersByTime(fullMove * 3));
    expect(result.current.rows).toEqual(initial);
    expect(result.current.behind).toBe(true);
    rerender({ entries: promoted, paused: false });
    act(() => vi.advanceTimersByTime(fullMove));
    expect(result.current.rows).toEqual(promoted);
  });

  it("applies reduced-motion updates instantly, including when disabled mid-climb", () => {
    const { result, rerender } = renderHook(
      ({ entries, animate }) => useScoreboard(entries, animate, false),
      { initialProps: { entries: initial, animate: true } },
    );
    rerender({ entries: promoted, animate: true });
    rerender({ entries: promoted, animate: false });
    expect(result.current.rows).toEqual(promoted);
    expect(result.current.changes).toEqual([]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("handles new competitors, removal, and empty snapshots without inventing places gained", () => {
    const { result, rerender, unmount } = renderHook(
      ({ entries }) => useScoreboard(entries, true, false),
      { initialProps: { entries: initial } },
    );
    const newcomer = [{ username: "new", rank: 1, solved: 4, penalty_s: 300 }];
    rerender({ entries: newcomer });
    expect(result.current.changes[0]).toMatchObject({ from: null, gained: 0, solved: true });
    act(() => vi.advanceTimersByTime(fullMove));
    expect(result.current.rows).toEqual(newcomer);
    rerender({ entries: [] });
    act(() => vi.advanceTimersByTime(fullMove));
    expect(result.current.rows).toEqual([]);
    rerender({ entries: initial });
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
