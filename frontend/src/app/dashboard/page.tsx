"use client";

import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";

import { AuthGate } from "@/components/auth/auth-gate";
import { ContestCard } from "@/components/dashboard/contest-card";
import { AppShell } from "@/components/shell/app-shell";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { apiFetch } from "@/lib/api";
import { staggerContainer } from "@/lib/motion";
import { queryKeys } from "@/lib/query-keys";
import { contestListSchema } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

const sections = [
  { filter: "active", title: "Active now", empty: "No contest is running right now." },
  { filter: "upcoming", title: "Upcoming", empty: "Nothing scheduled yet." },
  { filter: "past", title: "Previous", empty: "No finished contests yet." },
] as const;

function ContestSection({ filter, title, empty }: (typeof sections)[number]) {
  const query = useQuery({
    queryKey: queryKeys.contests(filter),
    queryFn: () => apiFetch(`/api/v1/contests?filter=${filter}`, { schema: contestListSchema }),
  });

  return (
    <section className="flex flex-col gap-3">
      <h2 className="font-display text-lg font-semibold">{title}</h2>

      {query.isPending && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-44 w-full rounded-[calc(var(--radius)+2px)]" />
          ))}
        </div>
      )}

      {query.isError && (
        <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
      )}

      {query.isSuccess &&
        (query.data.contests.length === 0 ? (
          <Card className="py-10 text-center text-sm text-faint">{empty}</Card>
        ) : (
          <motion.div
            variants={staggerContainer}
            initial="hidden"
            animate="show"
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          >
            {query.data.contests.map((c) => (
              <ContestCard key={c.id} contest={c} />
            ))}
          </motion.div>
        ))}
    </section>
  );
}

export default function DashboardPage() {
  const user = useAuthStore((s) => s.user);

  return (
    <AuthGate>
      <AppShell>
        <div className="mb-8">
          <h1 className="font-display text-2xl font-semibold tracking-tight">
            Welcome back,{" "}
            <span data-testid="current-user" className="text-primary">
              {user?.username}
            </span>
          </h1>
          <p className="mt-1 text-sm text-muted">
            Pick a contest and start solving — verdicts and rankings update live.
          </p>
        </div>

        <div className="flex flex-col gap-10">
          {sections.map((s) => (
            <ContestSection key={s.filter} {...s} />
          ))}
        </div>
      </AppShell>
    </AuthGate>
  );
}
