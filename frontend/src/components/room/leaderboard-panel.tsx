"use client";

import { useQuery } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";

import { Card } from "@/components/ui/card";
import { ErrorNotice } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { apiFetch } from "@/lib/api";
import { cn } from "@/lib/cn";
import { formatPenalty } from "@/lib/format";
import { spring } from "@/lib/motion";
import { queryKeys } from "@/lib/query-keys";
import { leaderboardSchema } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

const medal: Record<number, string> = {
  1: "text-amber-400",
  2: "text-slate-300",
  3: "text-orange-400",
};

export function LeaderboardPanel({ contestId }: { contestId: string }) {
  const username = useAuthStore((s) => s.user?.username);

  const query = useQuery({
    queryKey: queryKeys.leaderboard(contestId),
    queryFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/leaderboard`, { schema: leaderboardSchema }),
    // Live updates arrive over WS; this is the fallback cadence.
    refetchInterval: 60_000,
  });

  return (
    <Card className="overflow-hidden p-0" data-testid="leaderboard">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <h3 className="font-display text-sm font-semibold">Leaderboard</h3>
        <span className="text-xs text-faint">live</span>
      </div>

      {query.isPending && (
        <div className="space-y-2 p-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      )}

      {query.isError && (
        <div className="p-4">
          <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
        </div>
      )}

      {query.isSuccess &&
        (query.data.entries.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-faint">No solves yet — be the first.</p>
        ) : (
          <div>
            <div className="grid grid-cols-[2rem_1fr_3.5rem_5rem] gap-2 px-4 py-2 text-xs text-faint">
              <span>#</span>
              <span>User</span>
              <span className="text-right">Solved</span>
              <span className="text-right">Penalty</span>
            </div>
            <ul>
              <AnimatePresence initial={false}>
                {query.data.entries.map((row) => {
                  const me = row.username === username;
                  return (
                    <motion.li
                      key={row.username}
                      layout
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      transition={spring}
                      className={cn(
                        "grid grid-cols-[2rem_1fr_3.5rem_5rem] items-center gap-2 px-4 py-2 text-sm",
                        me
                          ? "bg-primary/10 font-medium text-foreground"
                          : "odd:bg-foreground/[0.02]",
                      )}
                    >
                      <span className={cn("font-mono", medal[row.rank] ?? "text-faint")}>
                        {row.rank}
                      </span>
                      <span className="truncate">
                        {row.username}
                        {me && <span className="ml-1.5 text-xs text-primary">you</span>}
                      </span>
                      <span className="text-right font-mono">{row.solved}</span>
                      <span className="text-right font-mono text-muted">
                        {formatPenalty(row.penalty_s)}
                      </span>
                    </motion.li>
                  );
                })}
              </AnimatePresence>
            </ul>
          </div>
        ))}
    </Card>
  );
}
