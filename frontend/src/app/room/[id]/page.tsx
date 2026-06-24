"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { AuthGate } from "@/components/auth/auth-gate";
import { EditorPanel } from "@/components/room/editor-panel";
import { LeaderboardPanel } from "@/components/room/leaderboard-panel";
import { ProblemPanel } from "@/components/room/problem-panel";
import { Badge, ErrorNotice, Spinner } from "@/components/ui/ui";
import { useContestEvents } from "@/hooks/use-contest-events";
import { apiFetch } from "@/lib/api";
import { contestPhase, formatCountdown } from "@/lib/format";
import { queryKeys } from "@/lib/query-keys";
import { contestDetailSchema } from "@/lib/schemas";

export default function RoomPage() {
  const params = useParams<{ id: string }>();
  return (
    <AuthGate>
      <Room contestId={params.id} />
    </AuthGate>
  );
}

function Room({ contestId }: { contestId: string }) {
  const { connected } = useContestEvents(contestId);
  const [selectedOrd, setSelectedOrd] = useState(1);

  const detail = useQuery({
    queryKey: queryKeys.contest(contestId),
    queryFn: () => apiFetch(`/api/v1/contests/${contestId}`, { schema: contestDetailSchema }),
  });

  // Entering a room registers participation (idempotent on the backend).
  const join = useMutation({
    mutationFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/join`, { method: "POST", auth: true }),
  });
  const joinMutate = join.mutate;
  useEffect(() => {
    joinMutate();
  }, [joinMutate]);

  if (detail.isPending) {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <Spinner />
      </main>
    );
  }
  if (detail.isError) {
    return (
      <main className="flex min-h-screen items-center justify-center px-6">
        <ErrorNotice message={detail.error.message} onRetry={() => void detail.refetch()} />
      </main>
    );
  }

  const { contest, problems } = detail.data;

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-screen-2xl flex-col gap-4 px-4 py-4 lg:px-6">
      <header className="flex flex-wrap items-center gap-3">
        <Link href="/dashboard" className="text-sm text-zinc-400 hover:text-zinc-200">
          ← Dashboard
        </Link>
        <h1 className="text-lg font-semibold" data-testid="contest-title">
          {contest.title}
        </h1>
        <ContestClock startsAt={contest.starts_at} endsAt={contest.ends_at} />
        <span className="ml-auto">
          {connected ? (
            <Badge tone="green" data-testid="ws-status">
              live
            </Badge>
          ) : (
            <Badge tone="amber" data-testid="ws-status">
              reconnecting…
            </Badge>
          )}
        </span>
      </header>

      <div className="grid flex-1 gap-4 lg:grid-cols-2">
        <div className="flex flex-col gap-4">
          <ProblemPanel
            contestId={contestId}
            problems={problems}
            selectedOrd={selectedOrd}
            onSelect={setSelectedOrd}
          />
          <LeaderboardPanel contestId={contestId} />
        </div>

        <EditorPanel
          contestId={contestId}
          problemOrd={selectedOrd}
          problemKey={`${contestId}:${selectedOrd}`}
        />
      </div>
    </main>
  );
}

function ContestClock({ startsAt, endsAt }: { startsAt: string; endsAt: string }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const phase = contestPhase(startsAt, endsAt, now);
  if (phase === "active") {
    return (
      <Badge tone="zinc" className="font-mono">
        ends in {formatCountdown(Date.parse(endsAt) - now)}
      </Badge>
    );
  }
  if (phase === "upcoming") {
    return (
      <Badge tone="sky" className="font-mono">
        starts in {formatCountdown(Date.parse(startsAt) - now)}
      </Badge>
    );
  }
  return <Badge tone="zinc">finished</Badge>;
}
