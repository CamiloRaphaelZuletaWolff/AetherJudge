"use client";

import { useQuery } from "@tanstack/react-query";

import { Card, ErrorNotice, Spinner } from "@/components/ui/ui";
import { apiFetch } from "@/lib/api";
import { formatPenalty } from "@/lib/format";
import { queryKeys } from "@/lib/query-keys";
import { leaderboardSchema } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

export function LeaderboardPanel({ contestId }: { contestId: string }) {
  const username = useAuthStore((s) => s.user?.username);

  const query = useQuery({
    queryKey: queryKeys.leaderboard(contestId),
    queryFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/leaderboard`, { schema: leaderboardSchema }),
    // Live updates arrive over WS; this is the fallback cadence.
    refetchInterval: 60_000,
  });

  if (query.isPending) {
    return (
      <Card className="flex justify-center py-8">
        <Spinner />
      </Card>
    );
  }
  if (query.isError) {
    return <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />;
  }

  const entries = query.data.entries;
  return (
    <Card className="p-0" data-testid="leaderboard">
      <h3 className="border-b border-zinc-800 px-4 py-3 text-sm font-semibold">Leaderboard</h3>
      {entries.length === 0 ? (
        <p className="px-4 py-6 text-center text-sm text-zinc-500">No solves yet — be the first.</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-xs text-zinc-500">
              <th className="px-4 py-2 font-medium">#</th>
              <th className="py-2 font-medium">User</th>
              <th className="py-2 text-right font-medium">Solved</th>
              <th className="px-4 py-2 text-right font-medium">Penalty</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((row) => (
              <tr
                key={row.username}
                className={
                  row.username === username
                    ? "bg-emerald-950/40 text-emerald-200"
                    : "odd:bg-zinc-900/40"
                }
              >
                <td className="px-4 py-2 font-mono text-zinc-500">{row.rank}</td>
                <td className="py-2 font-medium">{row.username}</td>
                <td className="py-2 text-right font-mono">{row.solved}</td>
                <td className="px-4 py-2 text-right font-mono text-zinc-400">
                  {formatPenalty(row.penalty_s)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Card>
  );
}
