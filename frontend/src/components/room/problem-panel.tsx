"use client";

import { useQuery } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";

import { Badge, Card, ErrorNotice, Spinner } from "@/components/ui/ui";
import { apiFetch } from "@/lib/api";
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
    <Card className="flex flex-col gap-4" data-testid="problem-panel">
      <div className="flex flex-wrap gap-2">
        {problems.map((p) => (
          <button
            key={p.ord}
            onClick={() => onSelect(p.ord)}
            data-testid={`problem-tab-${p.ord}`}
            className={`rounded-md px-3 py-1 text-sm font-medium transition-colors ${
              p.ord === selectedOrd
                ? "bg-emerald-700 text-white"
                : "bg-zinc-800 text-zinc-400 hover:text-zinc-200"
            }`}
          >
            {p.ord}. {p.title}
          </button>
        ))}
      </div>

      {query.isPending && (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      )}
      {query.isError && (
        <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />
      )}
      {query.isSuccess && (
        <div data-testid="problem-statement">
          <div className="mb-3 flex items-center gap-2">
            <h2 className="text-xl font-semibold">{query.data.title}</h2>
            <Badge tone="zinc">{query.data.time_limit_ms} ms</Badge>
            <Badge tone="zinc">{query.data.memory_limit_mb} MB</Badge>
          </div>
          <div className="prose-statement text-sm leading-relaxed text-zinc-300">
            <ReactMarkdown
              components={{
                h1: (props) => <h3 className="mt-4 mb-2 font-semibold text-zinc-100" {...props} />,
                h2: (props) => <h4 className="mt-4 mb-2 font-semibold text-zinc-100" {...props} />,
                strong: (props) => <strong className="text-zinc-100" {...props} />,
                p: (props) => <p className="mb-3" {...props} />,
                code: (props) => (
                  <code
                    className="rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-xs text-emerald-300"
                    {...props}
                  />
                ),
              }}
            >
              {query.data.statement_md}
            </ReactMarkdown>
          </div>
        </div>
      )}
    </Card>
  );
}
