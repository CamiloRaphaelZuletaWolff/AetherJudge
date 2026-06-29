"use client";

// Moderator/admin view of EVERY submission in a contest (RBAC
// submission.viewAll; ADR-0014). Rendered only behind <Can perm="..."> in the
// room, and backed by an endpoint the gateway enforces server-side.
import { useQuery } from "@tanstack/react-query";

import { VerdictBadge } from "@/components/room/verdict-badge";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { adminSubmissionListSchema } from "@/lib/schemas";

export function AllSubmissionsPanel({ contestId }: { contestId: string }) {
  const query = useQuery({
    queryKey: queryKeys.contestAllSubmissions(contestId),
    queryFn: () =>
      apiFetch(`/api/v1/admin/contests/${contestId}/submissions`, {
        schema: adminSubmissionListSchema,
        auth: true,
      }),
    // Keep it live while anything is still judging.
    refetchInterval: (q) =>
      q.state.data?.submissions.some((s) => s.status !== "done") ? 4_000 : false,
  });

  if (query.isPending) {
    return (
      <Card className="flex justify-center py-6">
        <Spinner />
      </Card>
    );
  }
  if (query.isError) {
    return <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />;
  }

  const submissions = query.data.submissions;
  return (
    <Card className="overflow-hidden p-0" data-testid="all-submissions">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <h3 className="font-display text-sm font-semibold">All submissions</h3>
        <Badge variant="info">Moderator view</Badge>
      </div>
      {submissions.length === 0 ? (
        <p className="px-4 py-5 text-center text-sm text-faint">No submissions yet.</p>
      ) : (
        <ul className="divide-y divide-border">
          {submissions.map((s) => (
            <li key={s.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
              <Avatar name={s.username} size="sm" />
              <span className="truncate font-medium text-foreground">{s.username}</span>
              <span className="font-mono text-xs text-faint">P{s.problem_ord}</span>
              <span className="text-muted">{s.language}</span>
              <span className="ml-auto hidden font-mono text-xs text-faint sm:inline">
                {new Date(s.submitted_at).toLocaleTimeString()}
              </span>
              <VerdictBadge submission={s} />
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
