"use client";

import { ArrowUpRight, Play } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Scoreboard } from "@/components/room/scoreboard";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { LeaderboardRow } from "@/lib/schemas";

const initial: LeaderboardRow[] = [
  { rank: 1, username: "byte_force", solved: 4, penalty_s: 5040 },
  { rank: 2, username: "runtime_heroes", solved: 3, penalty_s: 3960 },
  { rank: 3, username: "hello_world", solved: 3, penalty_s: 4620 },
];

export function ScoreboardDemo() {
  const [entries, setEntries] = useState(initial);
  const [playing, setPlaying] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  return (
    <Card className="overflow-hidden text-left">
      <div className="flex items-center justify-between border-b border-border px-4 py-4">
        <div className="flex items-center gap-2">
          <span className="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary">
            <ArrowUpRight className="size-4" />
          </span>
          <span className="font-display text-sm font-semibold">The next move is yours.</span>
        </div>
        <span className="rounded-md border border-border px-2 py-1 text-[10px] uppercase tracking-wider text-muted">
          Interactive demo
        </span>
      </div>
      <Scoreboard entries={entries} />
      <div className="border-t border-border bg-surface-2 p-4">
        <Button
          data-testid="scoreboard-demo-play"
          className="w-full"
          variant="secondary"
          disabled={playing}
          onClick={() => {
            setPlaying(true);
            setEntries((previous) => {
              const challenger = previous[previous.length - 1]?.username;
              const highScore = previous[0]?.solved ?? 0;
              const next = previous.map((row) =>
                row.username === challenger
                  ? { ...row, solved: highScore + 1, penalty_s: row.penalty_s + 600 }
                  : row,
              );
              return next
                .sort((a, b) => b.solved - a.solved || a.penalty_s - b.penalty_s)
                .map((row, index) => ({ ...row, rank: index + 1 }));
            });
            timer.current = setTimeout(() => setPlaying(false), 1500);
          }}
        >
          <Play className="size-3.5" />
          {playing ? "Climbing the leaderboard…" : "See an ICPC-style climb"}
        </Button>
      </div>
    </Card>
  );
}
