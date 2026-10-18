"use client";

import { useQuery } from "@tanstack/react-query";
import { Pause, Play, Search, Trophy, Zap } from "lucide-react";
import { useState } from "react";
import { Scoreboard } from "@/components/room/scoreboard";
import { Button } from "@/components/ui/button";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { leaderboardSchema } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

export function LeaderboardPanel({
  contestId,
  connected = false,
  totalProblems,
}: {
  contestId: string;
  connected?: boolean;
  totalProblems?: number;
}) {
  const username = useAuthStore((s) => s.user?.username);
  const [paused, setPaused] = useState(false);
  const [animations, setAnimations] = useState(true);
  const [search, setSearch] = useState("");
  const query = useQuery({
    queryKey: queryKeys.leaderboard(contestId),
    queryFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/leaderboard`, { schema: leaderboardSchema }),
    refetchInterval: connected ? 60_000 : 10_000,
  });
  return (
    <Card className="overflow-hidden p-0" data-testid="leaderboard">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-4">
        <div className="flex items-center gap-2.5">
          <span className="grid size-9 place-items-center rounded-lg bg-primary/10 text-primary">
            <Trophy className="size-4" />
          </span>
          <div>
            <h2 className="font-display text-sm font-semibold">Leaderboard</h2>
            <p className="mt-0.5 text-[11px] text-muted">
              {paused ? "Paused view" : connected ? "Live standings" : "Refreshing automatically"}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <Button
            size="icon"
            variant="ghost"
            aria-label={animations ? "Disable ranking animations" : "Enable ranking animations"}
            aria-pressed={animations}
            title="Ranking animations"
            onClick={() => setAnimations(!animations)}
          >
            <Zap className={animations ? "size-4 text-primary" : "size-4"} />
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setPaused(!paused)}
            aria-pressed={paused}
          >
            {paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
            {paused ? "Resume" : "Pause"}
          </Button>
        </div>
      </div>
      <div className="relative m-3">
        <Search className="pointer-events-none absolute left-3 top-3 size-4 text-faint" />
        <Input
          aria-label="Find a competitor"
          placeholder="Find a competitor…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-9"
        />
      </div>
      {query.isPending && (
        <div className="space-y-2 p-4" aria-label="Loading standings">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      )}
      {query.isError && (
        <div className="p-4">
          <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
        </div>
      )}
      {query.data && (
        <Scoreboard
          key={contestId}
          entries={query.data.entries}
          username={username}
          paused={paused}
          animations={animations}
          search={search}
          totalProblems={totalProblems}
        />
      )}
    </Card>
  );
}
