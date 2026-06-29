"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { AuthGate } from "@/components/auth/auth-gate";
import { Can } from "@/components/auth/can";
import { AllSubmissionsPanel } from "@/components/room/all-submissions-panel";
import { EditorPanel } from "@/components/room/editor-panel";
import { LeaderboardPanel } from "@/components/room/leaderboard-panel";
import { ProblemPanel } from "@/components/room/problem-panel";
import { UserMenu } from "@/components/shell/user-menu";
import { Badge } from "@/components/ui/badge";
import { ErrorNotice } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { ThemeToggle } from "@/components/ui/theme-toggle";
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
    <div className="min-h-screen">
      <header className="sticky top-0 z-40 border-b border-border bg-background/80 backdrop-blur-md">
        <div className="mx-auto flex h-14 w-full max-w-screen-2xl items-center gap-3 px-4 lg:px-6">
          <Link
            href="/dashboard"
            className="inline-flex items-center gap-1 text-sm text-muted transition-colors hover:text-foreground"
          >
            <ChevronLeft className="size-4" /> <span className="hidden sm:inline">Dashboard</span>
          </Link>
          <span className="h-5 w-px bg-border" />
          <h1 className="truncate font-display font-semibold" data-testid="contest-title">
            {contest.title}
          </h1>
          <ContestClock startsAt={contest.starts_at} endsAt={contest.ends_at} />
          <div className="ml-auto flex items-center gap-2">
            <Badge
              variant={connected ? "success" : "warning"}
              data-testid="ws-status"
              className={connected ? "animate-arena-ping" : undefined}
            >
              <span
                className={`size-1.5 rounded-full ${connected ? "bg-v-accepted" : "bg-v-tle"}`}
              />
              {connected ? "Live" : "Reconnecting…"}
            </Badge>
            <ThemeToggle />
            <UserMenu />
          </div>
        </div>
      </header>

      <main className="mx-auto grid w-full max-w-screen-2xl gap-4 px-4 py-4 lg:grid-cols-2 lg:px-6">
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
      </main>

      {/* Moderators and admins see everyone's submissions; plain users never
          render this (and the endpoint 403s them regardless). */}
      <Can perm="submission.viewAll">
        <section className="mx-auto w-full max-w-screen-2xl px-4 pb-6 lg:px-6">
          <AllSubmissionsPanel contestId={contestId} />
        </section>
      </Can>
    </div>
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
      <Badge variant="neutral" className="hidden font-mono sm:inline-flex">
        ends in {formatCountdown(Date.parse(endsAt) - now)}
      </Badge>
    );
  }
  if (phase === "upcoming") {
    return (
      <Badge variant="info" className="hidden font-mono sm:inline-flex">
        starts in {formatCountdown(Date.parse(startsAt) - now)}
      </Badge>
    );
  }
  return (
    <Badge variant="neutral" className="hidden sm:inline-flex">
      finished
    </Badge>
  );
}
