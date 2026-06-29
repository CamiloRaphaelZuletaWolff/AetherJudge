"use client";

import { useQuery } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";
import { ChevronDown, ChevronLeft, Cpu, FilePlus2, Pencil, Timer } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { ContestForm } from "@/components/admin/contest-form";
import { ProblemForm } from "@/components/admin/problem-form";
import { TestCaseEditor } from "@/components/admin/test-case-editor";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Markdown } from "@/components/ui/markdown";
import { Spinner } from "@/components/ui/spinner";
import { apiFetch } from "@/lib/api";
import { contestPhase, type ContestPhase } from "@/lib/format";
import { easeMove } from "@/lib/motion";
import { queryKeys } from "@/lib/query-keys";
import { adminProblemListSchema, contestDetailSchema, type AdminProblem } from "@/lib/schemas";

const phaseBadge: Record<ContestPhase, { label: string; variant: BadgeVariant }> = {
  active: { label: "Active", variant: "success" },
  upcoming: { label: "Upcoming", variant: "info" },
  past: { label: "Finished", variant: "neutral" },
};

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export function ContestBuilder({ contestId }: { contestId: string }) {
  const [editing, setEditing] = useState(false);

  const detail = useQuery({
    queryKey: queryKeys.contest(contestId),
    queryFn: () => apiFetch(`/api/v1/contests/${contestId}`, { schema: contestDetailSchema }),
  });

  const problems = useQuery({
    queryKey: queryKeys.adminContestProblems(contestId),
    queryFn: () =>
      apiFetch(`/api/v1/admin/contests/${contestId}/problems`, {
        schema: adminProblemListSchema,
        auth: true,
      }),
  });

  if (detail.isPending) {
    return (
      <div className="flex justify-center py-16">
        <Spinner />
      </div>
    );
  }
  if (detail.isError) {
    return <ErrorNotice message={detail.error.message} onRetry={() => void detail.refetch()} />;
  }

  const contest = detail.data.contest;
  const phase = phaseBadge[contestPhase(contest.starts_at, contest.ends_at)];

  return (
    <div className="flex flex-col gap-6">
      <Link
        href="/admin"
        className="inline-flex items-center gap-1 text-sm text-muted transition-colors hover:text-foreground"
      >
        <ChevronLeft className="size-4" /> Back to admin
      </Link>

      {/* Contest header / edit */}
      {editing ? (
        <ContestForm
          contest={contest}
          onSaved={() => {
            setEditing(false);
            void detail.refetch();
          }}
        />
      ) : (
        <Card className="flex flex-col gap-3 p-5">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <h1 className="font-display text-2xl font-semibold tracking-tight">
                  {contest.title}
                </h1>
                <Badge variant={phase.variant}>{phase.label}</Badge>
              </div>
              <p className="mt-1 font-mono text-xs text-faint">{contest.slug}</p>
            </div>
            <Button variant="secondary" size="sm" onClick={() => setEditing(true)}>
              <Pencil className="size-4" /> Edit
            </Button>
          </div>
          {contest.description && <p className="text-sm text-muted">{contest.description}</p>}
          <p className="text-sm text-muted">
            {fmtDate(contest.starts_at)} &rarr; {fmtDate(contest.ends_at)}
          </p>
        </Card>
      )}

      {/* Problems */}
      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="font-display text-lg font-semibold tracking-tight">Problems</h2>
          {problems.isSuccess && (
            <span className="text-sm text-faint">{problems.data.problems.length}</span>
          )}
        </div>

        {problems.isPending && (
          <Card className="flex justify-center py-8">
            <Spinner />
          </Card>
        )}
        {problems.isError && (
          <ErrorNotice message={problems.error.message} onRetry={() => void problems.refetch()} />
        )}
        {problems.isSuccess && problems.data.problems.length === 0 && (
          <Card className="p-5 text-sm text-faint">No problems yet. Add the first one below.</Card>
        )}
        {problems.isSuccess &&
          problems.data.problems.map((p) => (
            <ProblemRow key={p.id} contestId={contestId} problem={p} />
          ))}
      </section>

      {/* Add problem */}
      <section className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <FilePlus2 className="size-4 text-primary" />
          <h2 className="font-display text-lg font-semibold tracking-tight">Add a problem</h2>
        </div>
        <Card className="p-5">
          <ProblemForm contestId={contestId} />
        </Card>
      </section>
    </div>
  );
}

function ProblemRow({ contestId, problem }: { contestId: string; problem: AdminProblem }) {
  const [open, setOpen] = useState(false);
  const letter = String.fromCharCode(64 + problem.ord); // 1 -> A

  return (
    <Card className="overflow-hidden p-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-foreground/5"
      >
        <span className="grid size-7 shrink-0 place-items-center rounded-md bg-primary/10 font-mono text-sm font-semibold text-primary">
          {letter}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate font-medium text-foreground">{problem.title}</span>
        </span>
        <Badge variant="neutral" className="hidden sm:inline-flex">
          <Timer className="size-3" /> {problem.time_limit_ms} ms
        </Badge>
        <Badge variant="neutral" className="hidden sm:inline-flex">
          <Cpu className="size-3" /> {problem.memory_limit_mb} MB
        </Badge>
        <Badge variant={problem.test_case_count > 0 ? "success" : "warning"}>
          {problem.test_case_count} test{problem.test_case_count === 1 ? "" : "s"}
        </Badge>
        <ChevronDown
          className={`size-4 shrink-0 text-muted transition-transform ${open ? "rotate-180" : ""}`}
        />
      </button>

      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.22, ease: easeMove }}
            className="overflow-hidden"
          >
            <div className="flex flex-col gap-5 border-t border-border px-4 py-4">
              {problem.statement_md && (
                <div>
                  <h4 className="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">
                    Statement
                  </h4>
                  <div className="rounded-[var(--radius)] border border-border bg-surface-2 p-3">
                    <Markdown>{problem.statement_md}</Markdown>
                  </div>
                </div>
              )}
              <TestCaseEditor contestId={contestId} problemId={problem.id} />
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </Card>
  );
}
