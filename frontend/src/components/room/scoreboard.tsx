"use client";

import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { ArrowUp, Check, Trophy } from "lucide-react";
import { useScoreboard } from "@/hooks/use-scoreboard";
import { cn } from "@/lib/cn";
import { formatPenalty } from "@/lib/format";
import { easeMove } from "@/lib/motion";
import type { LeaderboardRow } from "@/lib/schemas";

export function Scoreboard({
  entries,
  username,
  paused = false,
  animations = true,
  search = "",
  totalProblems,
}: {
  entries: LeaderboardRow[];
  username?: string;
  paused?: boolean;
  animations?: boolean;
  search?: string;
  totalProblems?: number;
}) {
  const reduced = useReducedMotion();
  const animate = animations && !reduced;
  const { rows, changes, announcement, behind } = useScoreboard(entries, animate, paused);
  const changesByUser = new Map(changes.map((change) => [change.username, change]));
  const visible = rows.filter((row) => row.username.toLowerCase().includes(search.toLowerCase()));
  return (
    <div>
      <div
        className="flex min-h-11 items-center gap-2 border-b border-border bg-primary/5 px-4 py-2 text-xs text-muted"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        <span className="grid size-6 shrink-0 place-items-center rounded-full bg-primary/10 text-primary">
          <ArrowUp className="size-3.5" />
        </span>
        <span className="min-w-0 break-words">
          {paused
            ? behind
              ? "Standings paused · new updates waiting"
              : "Standings paused"
            : announcement || "Every solve counts. Watch the ranks change here."}
        </span>
      </div>
      <div className="overflow-x-auto" tabIndex={0} role="region" aria-label="Contest standings">
        <div role="table" aria-label="Leaderboard" className="min-w-[340px]">
          <div role="rowgroup" className="border-b border-border">
            <div
              role="row"
              className="scoreboard-grid px-4 py-3 text-[10px] font-semibold uppercase tracking-widest text-muted"
            >
              <span role="columnheader">Rank</span>
              <span role="columnheader">Competitor</span>
              <span role="columnheader" className="text-right">
                Solved
              </span>
              <span
                role="columnheader"
                className="text-right"
                title="Total contest penalty, displayed as hours:minutes:seconds"
              >
                Penalty
              </span>
            </div>
          </div>
          <div role="rowgroup" className="relative isolate">
            <AnimatePresence initial={false}>
              {visible.map((row) => {
                const change = changesByUser.get(row.username);
                const rising = (change?.gained ?? 0) > 0;
                const me = row.username === username;
                return (
                  <motion.div
                    role="row"
                    key={row.username}
                    layout={animate && !search ? "position" : false}
                    initial={animate ? { opacity: 0 } : false}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0, transition: { duration: animate ? 0.12 : 0 } }}
                    transition={{
                      layout: {
                        duration: Math.min(0.95, 0.6 + (change?.gained ?? 0) * 0.055),
                        ease: easeMove,
                      },
                      opacity: { duration: animate ? 0.18 : 0 },
                    }}
                    data-competitor={row.username}
                    data-rising={rising || undefined}
                    style={{
                      backgroundColor: change
                        ? "color-mix(in oklab, var(--primary) 15%, var(--surface))"
                        : me
                          ? "color-mix(in oklab, var(--primary) 5%, var(--surface))"
                          : "var(--surface)",
                    }}
                    className={cn(
                      "scoreboard-grid relative min-h-16 border-b border-border/60 bg-surface px-4 py-3 text-sm transition-[background-color] duration-200 last:border-b-0",
                      me && "bg-primary/5",
                      change && "z-10 bg-primary/15 ring-1 ring-inset ring-primary/35",
                    )}
                  >
                    <span
                      role="cell"
                      className="flex items-center gap-1 font-mono font-semibold tabular-nums"
                    >
                      {row.rank === 1 ? (
                        <Trophy
                          aria-label="First place"
                          className="size-4 shrink-0 text-amber-500"
                        />
                      ) : (
                        <span className="text-faint">#</span>
                      )}
                      {row.rank}
                    </span>
                    <span role="cell" className="flex min-w-0 items-center gap-2">
                      <span
                        className={cn(
                          "hidden size-8 shrink-0 items-center justify-center rounded-lg bg-foreground/5 font-display text-xs font-semibold sm:flex",
                          me && "bg-primary/15 text-primary",
                        )}
                        aria-hidden="true"
                      >
                        {row.username.slice(0, 2).toUpperCase()}
                      </span>
                      <span className="min-w-0">
                        <span className="block truncate font-medium" title={row.username}>
                          {row.username}
                        </span>
                        <span className="flex h-4 items-center gap-1 text-[10px] text-primary">
                          {rising ? (
                            <>
                              <ArrowUp className="size-3" />
                              {change!.gained} {change!.gained === 1 ? "place" : "places"}
                            </>
                          ) : change?.solved ? (
                            <>
                              <Check className="size-3" />
                              Accepted
                            </>
                          ) : me ? (
                            "You"
                          ) : row.rank === 1 ? (
                            "Leading the contest"
                          ) : (
                            ""
                          )}
                        </span>
                      </span>
                    </span>
                    <span role="cell" className="text-right">
                      <span className="inline-flex min-w-9 items-center justify-center gap-1 rounded-md bg-v-accepted/10 px-2 py-1 font-mono font-semibold tabular-nums text-v-accepted">
                        <span key={row.solved} className={cn(change?.solved && "score-number")}>
                          {row.solved}
                        </span>
                      </span>
                      {totalProblems !== undefined && (
                        <span className="mt-1 block text-[10px] text-faint">
                          of {totalProblems}
                        </span>
                      )}
                    </span>
                    <span
                      role="cell"
                      className="text-right font-mono text-xs tabular-nums text-muted"
                    >
                      {formatPenalty(row.penalty_s)}
                    </span>
                  </motion.div>
                );
              })}
            </AnimatePresence>
          </div>
        </div>
      </div>
      {visible.length === 0 && (
        <div className="px-5 py-12 text-center">
          <Trophy className="mx-auto mb-3 size-7 text-faint" />
          <p className="text-sm font-medium">
            {search ? "No matching competitors" : "The first solve is still up for grabs"}
          </p>
          <p className="mt-1 text-xs text-muted">
            {search ? "Try another username." : "Accepted submissions earn a place on the board."}
          </p>
        </div>
      )}
      <div className="flex flex-wrap justify-between gap-2 border-t border-border px-4 py-3 text-[11px] text-muted">
        <span>
          {rows.length} {rows.length === 1 ? "competitor" : "competitors"}
        </span>
        <span>Most solves first · lowest penalty breaks ties</span>
      </div>
    </div>
  );
}
