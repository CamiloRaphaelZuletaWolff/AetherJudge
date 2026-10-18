"use client";

import { useEffect, useRef, useState } from "react";
import { rankChanges, sameStandings, type RankChange } from "@/lib/leaderboard";
import type { LeaderboardRow } from "@/lib/schemas";

export const SCORE_REVEAL_MS = 280;
export const SCORE_SETTLE_MS = 1100;

// Reveal the score in its old position, then slide the same row to its new rank.
// While a move is playing, coalesce bursts to one latest snapshot (bounded memory).
export function useScoreboard(entries: LeaderboardRow[], animate: boolean, paused: boolean) {
  const [rows, setRows] = useState(entries);
  const [changes, setChanges] = useState<RankChange[]>([]);
  const [announcement, setAnnouncement] = useState("");
  const current = useRef(entries);
  const pending = useRef<LeaderboardRow[] | null>(null);
  const busy = useRef(false);
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);
  const pump = useRef<() => void>(() => {});

  useEffect(() => {
    const clear = () => {
      timers.current.forEach(clearTimeout);
      timers.current = [];
      busy.current = false;
    };
    pump.current = () => {
      if (busy.current || paused || !pending.current) return;
      const next = pending.current;
      pending.current = null;
      if (sameStandings(current.current, next)) return;
      const updates = rankChanges(current.current, next);
      const summary = updates
        .slice(0, 1)
        .map((change) =>
          change.gained > 0
            ? `${change.username} climbs ${change.gained} ${change.gained === 1 ? "place" : "places"} to #${change.to}`
            : `${change.username} solves a problem · #${change.to}`,
        )
        .join(". ");
      setAnnouncement(
        summary
          ? `${summary}${updates.length > 1 ? ` · ${updates.length - 1} more updates` : ""}`
          : "Standings updated.",
      );
      if (!animate) {
        current.current = next;
        setRows(next);
        setChanges([]);
        return;
      }
      busy.current = true;
      setChanges(updates);
      const byName = new Map(next.map((row) => [row.username, row]));
      setRows(
        current.current.map((row) => {
          const updated = byName.get(row.username);
          return updated ? { ...updated, rank: row.rank } : row;
        }),
      );
      timers.current.push(
        setTimeout(() => {
          current.current = next;
          setRows(next);
        }, SCORE_REVEAL_MS),
      );
      timers.current.push(
        setTimeout(() => {
          setChanges([]);
          clear();
          pump.current();
        }, SCORE_REVEAL_MS + SCORE_SETTLE_MS),
      );
    };
    // Pausing freezes the last complete standings, never a half-revealed score.
    clear();
    setRows(current.current);
    setChanges([]);
    pump.current();
    return clear;
  }, [animate, paused]);

  useEffect(() => {
    pending.current = entries;
    pump.current();
  }, [entries, animate, paused]);

  return { rows, changes, announcement, behind: !sameStandings(rows, entries) };
}
