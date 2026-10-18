"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { ChevronLeft, Clock, Code2, Trophy } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { memo, useEffect, useState } from "react";
import { AuthGate } from "@/components/auth/auth-gate";
import { Can } from "@/components/auth/can";
import { AllSubmissionsPanel } from "@/components/room/all-submissions-panel";
import { EditorPanel } from "@/components/room/editor-panel";
import { LeaderboardPanel } from "@/components/room/leaderboard-panel";
import { ProblemPanel } from "@/components/room/problem-panel";
import { UserMenu } from "@/components/shell/user-menu";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs } from "@/components/ui/tabs";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { useContestEvents } from "@/hooks/use-contest-events";
import { apiFetch } from "@/lib/api";
import { cn } from "@/lib/cn";
import { contestPhase, formatCountdown } from "@/lib/format";
import { queryKeys } from "@/lib/query-keys";
import { contestDetailSchema } from "@/lib/schemas";

// Clock ticks must not rerender Monaco, markdown, or all the animated rows.
const RoomEditor = memo(EditorPanel);
const RoomProblems = memo(ProblemPanel);
const RoomStandings = memo(LeaderboardPanel);

export default function RoomPage() {
  const params = useParams<{ id: string }>();
  return (
    <AuthGate>
      <Room key={params.id} contestId={params.id} />
    </AuthGate>
  );
}

function Room({ contestId }: { contestId: string }) {
  const { connected } = useContestEvents(contestId);
  const [selected, setSelected] = useState<number | null>(null);
  const [view, setView] = useState<"workspace" | "scoreboard">("workspace");
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const detail = useQuery({
    queryKey: queryKeys.contest(contestId),
    queryFn: () => apiFetch(`/api/v1/contests/${contestId}`, { schema: contestDetailSchema }),
  });
  const join = useMutation({
    mutationFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/join`, { method: "POST", auth: true }),
  });
  const joinMutate = join.mutate;
  useEffect(() => {
    joinMutate();
  }, [joinMutate]);
  if (detail.isPending)
    return (
      <main className="mx-auto max-w-screen-2xl space-y-6 p-6" aria-label="Loading contest">
        <Skeleton className="h-14 w-full" />
        <div className="grid gap-5 lg:grid-cols-2">
          <Skeleton className="h-96" />
          <Skeleton className="h-96" />
        </div>
      </main>
    );
  if (detail.isError)
    return (
      <main className="mx-auto max-w-xl space-y-6 p-8">
        <Link href="/dashboard" className="text-sm text-primary">
          ← Back to contests
        </Link>
        <ErrorNotice message={detail.error.message} onRetry={() => void detail.refetch()} />
      </main>
    );
  const { contest, problems } = detail.data;
  const selectedOrd =
    selected !== null && problems.some((p) => p.ord === selected) ? selected : problems[0]?.ord;
  const phase = contestPhase(contest.starts_at, contest.ends_at, now);
  const progress = Math.max(
    0,
    Math.min(
      1,
      (now - Date.parse(contest.starts_at)) /
        (Date.parse(contest.ends_at) - Date.parse(contest.starts_at)),
    ),
  );
  return (
    <div className="min-h-screen">
      <a href="#room-content" className="skip-link">
        Skip to contest
      </a>
      <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur-md">
        <div className="mx-auto flex min-h-16 w-full max-w-screen-2xl items-center gap-2 px-3 sm:gap-3 lg:px-6">
          <Link
            href="/dashboard"
            aria-label="Back to contests"
            className="grid size-10 shrink-0 place-items-center rounded-lg border border-border text-muted transition-colors hover:bg-surface hover:text-foreground"
          >
            <ChevronLeft className="size-4" />
          </Link>
          <div className="min-w-0 flex-1">
            <p className="eyebrow hidden sm:block">Contest arena</p>
            <h1
              className="truncate font-display text-sm font-semibold sm:text-base"
              data-testid="contest-title"
            >
              {contest.title}
            </h1>
          </div>
          <div className="flex shrink-0 items-center gap-1 sm:gap-2">
            <Badge variant={connected ? "success" : "warning"} data-testid="ws-status">
              <span
                className={cn("size-1.5 rounded-full", connected ? "bg-v-accepted" : "bg-v-tle")}
              />
              {connected ? "Live" : "Reconnecting…"}
            </Badge>
            <ThemeToggle />
            <UserMenu />
          </div>
        </div>
        <div
          className="h-0.5 bg-border"
          role="progressbar"
          aria-label="Contest elapsed"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(progress * 100)}
        >
          <div
            className="h-full origin-left bg-primary"
            style={{ transform: `scaleX(${progress})` }}
          />
        </div>
      </header>
      <main
        id="room-content"
        tabIndex={-1}
        className="mx-auto w-full max-w-screen-2xl px-3 py-4 sm:px-4 lg:px-6"
      >
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <Tabs
            value={view}
            onChange={setView}
            label="Contest view"
            options={[
              { value: "workspace", label: "Workspace" },
              { value: "scoreboard", label: "Full scoreboard" },
            ]}
          />
          <div className="flex items-center gap-3 text-xs text-muted">
            <span className="hidden items-center gap-1.5 sm:flex">
              <Code2 className="size-3.5" />
              {problems.length} problems
            </span>
            <span
              className={cn(
                "flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2 font-mono tabular-nums",
                phase === "active" &&
                  Date.parse(contest.ends_at) - now < 300000 &&
                  "border-v-tle/30 text-v-tle",
              )}
            >
              <Clock className="size-3.5" />
              {phase === "past"
                ? "Contest finished"
                : phase === "upcoming"
                  ? `Starts in ${formatCountdown(Date.parse(contest.starts_at) - now)}`
                  : `${formatCountdown(Date.parse(contest.ends_at) - now)} remaining`}
            </span>
          </div>
        </div>
        {join.isError && (
          <div className="mb-4">
            <ErrorNotice
              message={`Couldn't join this contest. ${join.error.message}`}
              onRetry={() => join.mutate()}
            />
          </div>
        )}
        {phase !== "active" && (
          <div className="mb-4 flex items-center gap-3 rounded-xl border border-border bg-surface p-4 text-sm text-muted">
            <Trophy className="size-5 shrink-0 text-primary" />
            {phase === "past"
              ? "The contest has ended. Explore the problems, test your code, and review the final standings."
              : "The contest hasn't started yet. Get your workspace ready; submissions open when the timer begins."}
          </div>
        )}
        <div
          className={cn(
            "grid items-start gap-4",
            view === "workspace" && "lg:grid-cols-[minmax(0,.95fr)_minmax(0,1.05fr)]",
          )}
        >
          <div className={cn("min-w-0", view === "scoreboard" && "hidden")}>
            {selectedOrd !== undefined ? (
              <RoomProblems
                contestId={contestId}
                problems={problems}
                selectedOrd={selectedOrd}
                onSelect={setSelected}
              />
            ) : (
              <Card className="p-8 text-center text-sm text-muted">
                No problems have been added yet.
              </Card>
            )}
            <a
              href="#solution-editor"
              className="mt-3 inline-flex items-center gap-2 text-sm text-primary lg:hidden"
            >
              <Code2 className="size-4" /> Go to editor ↓
            </a>
          </div>
          <div
            id="solution-editor"
            className={cn(
              "min-w-0 scroll-mt-24 lg:col-start-2 lg:row-span-2 lg:row-start-1",
              view === "scoreboard" && "hidden",
            )}
          >
            {selectedOrd !== undefined && (
              <RoomEditor
                key={selectedOrd}
                contestId={contestId}
                problemOrd={selectedOrd}
                problemKey={`${contestId}:${selectedOrd}`}
                submissionDisabledReason={
                  phase !== "active"
                    ? phase === "past"
                      ? "Submissions closed"
                      : "Contest not started"
                    : !join.isSuccess
                      ? "Joining contest…"
                      : undefined
                }
              />
            )}
          </div>
          <div className={cn("min-w-0", view === "scoreboard" && "mx-auto w-full max-w-5xl")}>
            {view === "scoreboard" && (
              <div className="mb-5 flex items-center justify-between gap-4">
                <div>
                  <p className="eyebrow mb-2">Follow the competition</p>
                  <h2 className="font-display text-2xl font-semibold">Every position matters.</h2>
                </div>
                <Button variant="secondary" size="sm" onClick={() => setView("workspace")}>
                  Back to solving
                </Button>
              </div>
            )}
            <RoomStandings
              contestId={contestId}
              connected={connected}
              totalProblems={problems.length}
            />
          </div>
        </div>
      </main>
      <Can perm="submission.viewAll">
        <section className="mx-auto w-full max-w-screen-2xl px-4 pb-6 lg:px-6">
          <AllSubmissionsPanel contestId={contestId} />
        </section>
      </Can>
    </div>
  );
}
