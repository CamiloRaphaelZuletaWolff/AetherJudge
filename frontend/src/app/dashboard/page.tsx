"use client";

import { useQuery } from "@tanstack/react-query";
import { CalendarClock, Search, Trophy, Radio, ArrowRight, X } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { AuthGate } from "@/components/auth/auth-gate";
import { Can } from "@/components/auth/can";
import { ContestCard } from "@/components/dashboard/contest-card";
import { AppShell } from "@/components/shell/app-shell";
import { Button, buttonStyles } from "@/components/ui/button";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs } from "@/components/ui/tabs";
import { apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { contestListSchema } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

const sections = [
  {
    filter: "active",
    title: "Live now",
    empty: "A quiet moment before the next challenge.",
    icon: Radio,
  },
  {
    filter: "upcoming",
    title: "Coming up",
    empty: "New contests will appear here when scheduled.",
    icon: CalendarClock,
  },
  {
    filter: "past",
    title: "Past contests",
    empty: "Completed contests and their standings will appear here.",
    icon: Trophy,
  },
] as const;
type Filter = "all" | "active" | "upcoming" | "past";

function ContestSection({
  filter,
  title,
  empty,
  icon: Icon,
  search,
}: (typeof sections)[number] & { search: string }) {
  const query = useQuery({
    queryKey: queryKeys.contests(filter),
    queryFn: () => apiFetch(`/api/v1/contests?filter=${filter}`, { schema: contestListSchema }),
    refetchInterval: 30_000,
  });
  const contests =
    query.data?.contests.filter((c) =>
      (c.title + " " + c.description).toLowerCase().includes(search.toLowerCase()),
    ) ?? [];
  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center gap-2.5">
        <Icon className={filter === "active" ? "size-4 text-primary" : "size-4 text-muted"} />
        <h2 className="font-display text-lg font-semibold">{title}</h2>
        {query.data && (
          <span className="rounded-md bg-foreground/5 px-2 py-0.5 font-mono text-xs text-muted">
            {contests.length}
          </span>
        )}
        <span className="ml-2 h-px flex-1 bg-border" />
      </div>
      {query.isPending && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-60 w-full rounded-2xl" />
          ))}
        </div>
      )}
      {query.isError && (
        <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
      )}
      {query.isSuccess &&
        (contests.length === 0 ? (
          <Card className="flex items-center gap-4 border-dashed bg-transparent p-6 shadow-none">
            <Icon className="size-6 shrink-0 text-faint" />
            <div>
              <p className="text-sm font-medium">
                {search
                  ? "No contests match your search"
                  : filter === "active"
                    ? "No live contests right now"
                    : filter === "upcoming"
                      ? "Nothing scheduled yet"
                      : "No finished contests yet"}
              </p>
              <p className="mt-1 text-sm text-muted">
                {search ? "Try a different name or clear the search." : empty}
              </p>
            </div>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {contests.map((c) => (
              <ContestCard key={c.id} contest={c} />
            ))}
          </div>
        ))}
    </section>
  );
}

export default function DashboardPage() {
  const user = useAuthStore((s) => s.user);
  const [filter, setFilter] = useState<Filter>("all");
  const [search, setSearch] = useState("");
  return (
    <AuthGate>
      <AppShell>
        <section className="relative mb-8 overflow-hidden rounded-2xl border border-primary/20 bg-surface px-6 py-8 sm:px-8">
          <div
            aria-hidden="true"
            className="bg-grid pointer-events-none absolute inset-y-0 right-0 w-1/2 opacity-40 [mask-image:linear-gradient(to_right,transparent,black)]"
          />
          <div className="relative flex flex-wrap items-center justify-between gap-6">
            <div>
              <p className="eyebrow mb-3">Your next challenge starts here</p>
              <h1 className="font-display text-3xl font-semibold tracking-tight sm:text-4xl">
                Ready,{" "}
                <span data-testid="current-user" className="text-primary">
                  {user?.username}
                </span>
                ?
              </h1>
              <p className="mt-3 max-w-lg text-sm leading-relaxed text-muted">
                Find your contest. Solve something new. Make your next move up the leaderboard.
              </p>
            </div>
            <div className="hidden items-center gap-3 rounded-xl border border-border bg-background/70 p-4 sm:flex">
              <span className="grid size-11 place-items-center rounded-xl bg-primary/10 text-primary">
                <Trophy className="size-5" />
              </span>
              <div>
                <p className="text-sm font-medium">One solve can change everything.</p>
                <p className="mt-1 text-xs text-muted">C++ · Python · Go</p>
              </div>
            </div>
          </div>
        </section>
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h2 className="font-display text-2xl font-semibold tracking-tight">Explore contests</h2>
            <p className="mt-1 text-sm text-muted">
              Join the action or revisit the final standings.
            </p>
          </div>
          <Can perm="admin.access">
            <Link
              href="/admin/contests/new"
              className={buttonStyles({ variant: "secondary", size: "sm" })}
            >
              Create contest <ArrowRight className="size-3.5" />
            </Link>
          </Can>
        </div>
        <div className="mb-8 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Tabs
            value={filter}
            onChange={setFilter}
            label="Contest status"
            options={[
              { value: "all", label: "All contests" },
              { value: "active", label: "Live" },
              { value: "upcoming", label: "Upcoming" },
              { value: "past", label: "Finished" },
            ]}
          />
          <div className="relative w-full sm:max-w-72">
            <Search className="pointer-events-none absolute left-3 top-3 size-4 text-faint" />
            <Input
              aria-label="Search contests"
              placeholder="Search contests…"
              className="pl-9 pr-10"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {search && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-0.5 top-0.5"
                onClick={() => setSearch("")}
                aria-label="Clear search"
              >
                <X className="size-3.5" />
              </Button>
            )}
          </div>
        </div>
        <div className="flex flex-col gap-9">
          {sections
            .filter((s) => filter === "all" || s.filter === filter)
            .map((s) => (
              <ContestSection key={s.filter} {...s} search={search} />
            ))}
        </div>
      </AppShell>
    </AuthGate>
  );
}
