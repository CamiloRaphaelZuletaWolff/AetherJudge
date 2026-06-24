"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import { useState } from "react";

import { VerdictBadge } from "@/components/room/verdict-badge";
import { Button, Card, ErrorNotice, Spinner } from "@/components/ui/ui";
import { ApiError, apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import {
  languageSchema,
  runResponseSchema,
  submissionListSchema,
  submissionSchema,
  type Language,
  type RunResponse,
} from "@/lib/schemas";
import { useEditorStore } from "@/stores/editor";

// Monaco is megabytes — load it only here, only in the browser.
const MonacoEditor = dynamic(() => import("@monaco-editor/react"), {
  ssr: false,
  loading: () => (
    <div className="flex h-full items-center justify-center">
      <Spinner />
    </div>
  ),
});

const monacoLanguage: Record<Language, string> = {
  cpp: "cpp",
  python: "python",
  go: "go",
};

const languageLabels: Record<Language, string> = {
  cpp: "C++",
  python: "Python",
  go: "Go",
};

export function EditorPanel({
  contestId,
  problemOrd,
  problemKey,
}: {
  contestId: string;
  problemOrd: number;
  // problemKey scopes drafts: unique per contest+problem.
  problemKey: string;
}) {
  const queryClient = useQueryClient();
  const { language, setLanguage, getDraft, setDraft } = useEditorStore();
  const code = getDraft(problemKey, language);

  const [stdin, setStdin] = useState("");
  const [runResult, setRunResult] = useState<RunResponse | null>(null);

  const run = useMutation({
    mutationFn: () =>
      apiFetch("/api/v1/run", {
        method: "POST",
        body: { language, code, stdin },
        schema: runResponseSchema,
        auth: true,
      }),
    onSuccess: setRunResult,
  });

  const submit = useMutation({
    mutationFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/problems/${problemOrd}/submissions`, {
        method: "POST",
        body: { language, code },
        schema: submissionSchema,
        auth: true,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.mySubmissions(contestId) });
    },
  });

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex items-center gap-2">
        <select
          value={language}
          onChange={(e) => setLanguage(languageSchema.parse(e.target.value))}
          data-testid="language-select"
          className="rounded-md border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-sm"
          aria-label="Language"
        >
          {(Object.keys(languageLabels) as Language[]).map((l) => (
            <option key={l} value={l}>
              {languageLabels[l]}
            </option>
          ))}
        </select>

        <div className="ml-auto flex gap-2">
          <Button
            variant="secondary"
            loading={run.isPending}
            onClick={() => run.mutate()}
            data-testid="run-button"
          >
            Run code
          </Button>
          <Button
            loading={submit.isPending}
            onClick={() => submit.mutate()}
            data-testid="submit-button"
          >
            Submit
          </Button>
        </div>
      </div>

      <div className="min-h-[320px] flex-1 overflow-hidden rounded-lg border border-zinc-800">
        <MonacoEditor
          height="100%"
          theme="vs-dark"
          language={monacoLanguage[language]}
          value={code}
          onChange={(value) => setDraft(problemKey, language, value ?? "")}
          options={{
            minimap: { enabled: false },
            fontSize: 14,
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 4,
          }}
        />
      </div>

      <Card className="flex flex-col gap-2 p-3">
        <label htmlFor="custom-stdin" className="text-xs font-medium text-zinc-400">
          Custom input (stdin) for Run
        </label>
        <textarea
          id="custom-stdin"
          value={stdin}
          onChange={(e) => setStdin(e.target.value)}
          data-testid="stdin-input"
          rows={2}
          className="resize-y rounded-md border border-zinc-700 bg-zinc-900 px-3 py-2 font-mono text-sm outline-none focus:border-emerald-600"
          placeholder="input piped to your program"
        />
      </Card>

      {submit.isError && (
        <ErrorNotice
          message={
            submit.error instanceof ApiError && submit.error.code === "rate_limited"
              ? "You're submitting too fast — wait a moment."
              : submit.error.message
          }
        />
      )}
      {submit.isSuccess && (
        <p className="text-sm text-zinc-400" data-testid="submit-confirmation">
          Submitted — watch the verdict below.
        </p>
      )}

      {run.isError && <ErrorNotice message={run.error.message} />}
      {runResult && <RunOutput result={runResult} />}

      <SubmissionsList contestId={contestId} />
    </div>
  );
}

function RunOutput({ result }: { result: RunResponse }) {
  const statusLabel: Record<RunResponse["status"], string> = {
    ok: `exited ${result.exit_code} in ${result.time_used_ms} ms`,
    compile_error: "compilation failed",
    runtime_error: `runtime error (exit ${result.exit_code})`,
    timeout: "time limit exceeded",
    memory_exceeded: "memory limit exceeded",
    error: "execution failed — try again",
  };

  return (
    <Card className="flex flex-col gap-2 p-3" data-testid="run-output">
      <p className="text-xs font-medium text-zinc-400">Run result: {statusLabel[result.status]}</p>
      {result.compile_output && (
        <pre className="overflow-x-auto rounded bg-zinc-900 p-2 font-mono text-xs text-sky-300">
          {result.compile_output}
        </pre>
      )}
      {result.stdout && (
        <pre
          className="overflow-x-auto rounded bg-zinc-900 p-2 font-mono text-xs text-zinc-200"
          data-testid="run-stdout"
        >
          {result.stdout}
        </pre>
      )}
      {result.stderr && (
        <pre className="overflow-x-auto rounded bg-zinc-900 p-2 font-mono text-xs text-orange-300">
          {result.stderr}
        </pre>
      )}
      {!result.stdout && !result.stderr && !result.compile_output && (
        <p className="text-xs text-zinc-600">(no output)</p>
      )}
    </Card>
  );
}

function SubmissionsList({ contestId }: { contestId: string }) {
  const query = useQuery({
    queryKey: queryKeys.mySubmissions(contestId),
    queryFn: () =>
      apiFetch(`/api/v1/contests/${contestId}/submissions`, {
        schema: submissionListSchema,
        auth: true,
      }),
    // WS invalidation drives freshness; poll as a safety net while pending.
    refetchInterval: (q) =>
      q.state.data?.submissions.some((s) => s.status !== "done") ? 2_000 : false,
  });

  if (query.isPending) {
    return (
      <Card className="flex justify-center py-6">
        <Spinner />
      </Card>
    );
  }
  if (query.isError) {
    return <ErrorNotice message={query.error.message} onRetry={() => void query.refetch()} />;
  }

  const submissions = query.data.submissions;
  return (
    <Card className="p-0" data-testid="my-submissions">
      <h3 className="border-b border-zinc-800 px-4 py-3 text-sm font-semibold">My submissions</h3>
      {submissions.length === 0 ? (
        <p className="px-4 py-5 text-center text-sm text-zinc-500">Nothing submitted yet.</p>
      ) : (
        <ul className="divide-y divide-zinc-800/60">
          {submissions.map((s) => (
            <li key={s.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
              <span className="font-mono text-xs text-zinc-500">P{s.problem_ord}</span>
              <span className="text-zinc-400">{s.language}</span>
              <span className="font-mono text-xs text-zinc-600">
                {new Date(s.submitted_at).toLocaleTimeString()}
              </span>
              <span className="ml-auto">
                <VerdictBadge submission={s} />
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
