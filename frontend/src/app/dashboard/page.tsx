"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";

import { AuthGate } from "@/components/auth/auth-gate";
import { ContestCard } from "@/components/dashboard/contest-card";
import { Button, Card, ErrorNotice, Spinner } from "@/components/ui/ui";
import { apiFetch } from "@/lib/api";
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
      <h2 className="text-lg font-semibold">{title}</h2>

      {query.isPending && (
        <Card className="flex items-center justify-center py-10">
          <Spinner />
        </Card>
      )}

      {query.isError && (
        <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
      )}

      {query.isSuccess &&
        (query.data.contests.length === 0 ? (
          <Card className="py-8 text-center text-sm text-zinc-500">{empty}</Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {query.data.contests.map((c) => (
              <ContestCard key={c.id} contest={c} />
            ))}
          </div>
        ))}
    </section>
  );
}

export default function DashboardPage() {
  const user = useAuthStore((s) => s.user);
  const signOut = useAuthStore((s) => s.signOut);

  return (
    <AuthGate>
      <main className="mx-auto flex min-h-screen w-full max-w-5xl flex-col gap-8 px-6 py-10">
        <header className="flex items-center justify-between">
          <Link href="/" className="text-2xl font-bold tracking-tight">
            Arena
          </Link>
          <div className="flex items-center gap-3">
            <span className="text-sm text-zinc-400" data-testid="current-user">
              {user?.username}
            </span>
            <Button variant="ghost" onClick={() => void signOut()}>
              Sign out
            </Button>
          </div>
        </header>

        {sections.map((s) => (
          <ContestSection key={s.filter} {...s} />
        ))}
      </main>
    </AuthGate>
  );
}
