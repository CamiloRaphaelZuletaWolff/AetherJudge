"use client";

import { useQuery } from "@tanstack/react-query";
import { Cpu, Timer } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { ErrorNotice } from "@/components/ui/card";
import { Markdown } from "@/components/ui/markdown";
import { Skeleton } from "@/components/ui/skeleton";
import { apiFetch } from "@/lib/api";
import { cn } from "@/lib/cn";
import { queryKeys } from "@/lib/query-keys";
import { problemSchema, type ProblemSummary } from "@/lib/schemas";

export function ProblemPanel({
  contestId,
  problems,
  selectedOrd,
  onSelect,
}: {
  contestId: string;
  problems: ProblemSummary[];
  selectedOrd: number;
  onSelect: (ord: number) => void;
}) {
  const query = useQuery({
    queryKey: queryKeys.problem(contestId, selectedOrd),
    queryFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/problems/${selectedOrd}`, {
        schema: problemSchema,
        auth: true,
      }),
  });

  return (
    <Card className="flex flex-col gap-4 p-5" data-testid="problem-panel">
      <div className="flex flex-wrap gap-1.5">
        {problems.map((p) => {
          const active = p.ord === selectedOrd;
          return (
            <button
              key={p.ord}
              onClick={() => onSelect(p.ord)}
              data-testid={`problem-tab-${p.ord}`}
              className={cn(
                "cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                active
                  ? "bg-primary text-primary-foreground"
                  : "bg-surface-2 text-muted hover:text-foreground",
              )}
            >
              <span className="font-mono text-xs opacity-70">{p.ord}</span>{" "}
              <span className="hidden sm:inline">{p.title}</span>
            </button>
          );
        })}
      </div>

      {query.isPending && (
        <div className="space-y-3">
          <Skeleton className="h-6 w-2/3" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-11/12" />
          <Skeleton className="h-4 w-4/5" />
        </div>
      )}
      {query.isError && (
        <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
      )}
      {query.isSuccess && (
        <div data-testid="problem-statement">
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <h2 className="font-display text-xl font-semibold">{query.data.title}</h2>
            <Badge variant="neutral">
              <Timer className="size-3" /> {query.data.time_limit_ms} ms
            </Badge>
            <Badge variant="neutral">
              <Cpu className="size-3" /> {query.data.memory_limit_mb} MB
            </Badge>
          </div>
          <Markdown>{query.data.statement_md}</Markdown>
        </div>
      )}
    </Card>
  );
}
