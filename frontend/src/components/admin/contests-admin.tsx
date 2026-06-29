"use client";

import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { Plus } from "lucide-react";
import Link from "next/link";

import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { apiFetch } from "@/lib/api";
import { contestPhase, type ContestPhase } from "@/lib/format";
import { fadeInUp, staggerContainer } from "@/lib/motion";
import { queryKeys } from "@/lib/query-keys";
import { contestListSchema } from "@/lib/schemas";

const phaseBadge: Record<ContestPhase, { label: string; variant: BadgeVariant }> = {
  active: { label: "Active", variant: "success" },
  upcoming: { label: "Upcoming", variant: "info" },
  past: { label: "Finished", variant: "neutral" },
};

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export function ContestsAdmin() {
  const contests = useQuery({
    queryKey: queryKeys.contests("all"),
    queryFn: () => apiFetch("/api/v1/contests?filter=all", { schema: contestListSchema }),
  });

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm text-muted">
          Create and schedule contests, then add problems and hidden test cases.
        </p>
        <Link href="/admin/contests/new">
          <Button size="sm" data-testid="new-contest">
            <Plus className="size-4" /> New contest
          </Button>
        </Link>
      </div>

      {contests.isPending && (
        <div className="grid gap-3 sm:grid-cols-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 w-full" />
          ))}
        </div>
      )}
      {contests.isError && (
        <ErrorNotice message={contests.error.message} onRetry={() => void contests.refetch()} />
      )}
      {contests.isSuccess && contests.data.contests.length === 0 && (
        <Card className="p-6 text-center text-sm text-faint">
          No contests yet. Create your first one.
        </Card>
      )}
      {contests.isSuccess && contests.data.contests.length > 0 && (
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="show"
          className="grid gap-3 sm:grid-cols-2"
        >
          {contests.data.contests.map((c) => {
            const phase = phaseBadge[contestPhase(c.starts_at, c.ends_at)];
            return (
              <motion.div key={c.id} variants={fadeInUp}>
                <Link href={`/admin/contests/${c.id}`} className="block">
                  <Card className="flex h-full flex-col gap-2 p-4 transition-colors hover:border-primary/50">
                    <div className="flex items-start justify-between gap-2">
                      <h3 className="font-display font-semibold">{c.title}</h3>
                      <Badge variant={phase.variant}>{phase.label}</Badge>
                    </div>
                    <p className="font-mono text-xs text-faint">{c.slug}</p>
                    <p className="mt-auto text-xs text-muted">
                      {fmtDate(c.starts_at)} &rarr; {fmtDate(c.ends_at)}
                    </p>
                  </Card>
                </Link>
              </motion.div>
            );
          })}
        </motion.div>
      )}
    </div>
  );
}
