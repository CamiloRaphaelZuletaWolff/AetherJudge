"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Play, Send } from "lucide-react";
import dynamic from "next/dynamic";
import { useState } from "react";

import { VerdictBadge } from "@/components/room/verdict-badge";
import { useTheme } from "@/components/theme/theme-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ErrorNotice } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/input";
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

const monacoLanguage: Record<Language, string> = { cpp: "cpp", python: "python", go: "go" };
const languageLabels: Record<Language, string> = { cpp: "C++", python: "Python", go: "Go" };

export function EditorPanel({
  contestId,
  problemOrd,
  problemKey,
}: {
  contestId: string;
  problemOrd: number;
  problemKey: string;
}) {
  const queryClient = useQueryClient();
  const { theme } = useTheme();
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
        <div className="relative">
          <select
            value={language}
            onChange={(e) => setLanguage(languageSchema.parse(e.target.value))}
            data-testid="language-select"
            aria-label="Language"
            className="cursor-pointer rounded-[var(--radius)] border border-border-strong bg-surface-2 px-3 py-1.5 text-sm font-medium text-foreground outline-none focus:border-primary"
          >
            {(Object.keys(languageLabels) as Language[]).map((l) => (
              <option key={l} value={l}>
                {languageLabels[l]}
              </option>
            ))}
          </select>
        </div>

        <div className="ml-auto flex gap-2">
          <Button
            variant="secondary"
            loading={run.isPending}
            onClick={() => run.mutate()}
            data-testid="run-button"
          >
            {!run.isPending && <Play className="size-4" />} Run
          </Button>
          <Button
            loading={submit.isPending}
            onClick={() => submit.mutate()}
            data-testid="submit-button"
          >
            {!submit.isPending && <Send className="size-4" />} Submit
          </Button>
        </div>
      </div>

      <div className="min-h-[320px] flex-1 overflow-hidden rounded-[calc(var(--radius)+2px)] border border-border bg-surface">
        <MonacoEditor
          height="100%"
          theme={theme === "dark" ? "vs-dark" : "light"}
          language={monacoLanguage[language]}
          value={code}
          onChange={(value) => setDraft(problemKey, language, value ?? "")}
          options={{
            minimap: { enabled: false },
            fontSize: 14,
            fontFamily: "var(--font-jetbrains-mono), monospace",
            scrollBeyondLastLine: false,
            automaticLayout: true,
            padding: { top: 12 },
            tabSize: 4,
          }}
        />
      </div>

      <Card className="flex flex-col gap-2 p-3">
        <label htmlFor="custom-stdin" className="text-xs font-medium text-muted">
          Custom input (stdin) for Run
        </label>
        <Textarea
          id="custom-stdin"
          value={stdin}
          onChange={(e) => setStdin(e.target.value)}
          data-testid="stdin-input"
          rows={2}
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
        <p className="text-sm text-muted" data-testid="submit-confirmation">
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
      <p className="text-xs font-medium text-muted">Run result: {statusLabel[result.status]}</p>
      {result.compile_output && (
        <pre className="overflow-x-auto rounded-md bg-surface-2 p-2 font-mono text-xs text-v-ce">
          {result.compile_output}
        </pre>
      )}
      {result.stdout && (
        <pre
          className="overflow-x-auto rounded-md bg-surface-2 p-2 font-mono text-xs text-foreground"
          data-testid="run-stdout"
        >
          {result.stdout}
        </pre>
      )}
      {result.stderr && (
        <pre className="overflow-x-auto rounded-md bg-surface-2 p-2 font-mono text-xs text-v-re">
          {result.stderr}
        </pre>
      )}
      {!result.stdout && !result.stderr && !result.compile_output && (
        <p className="text-xs text-faint">(no output)</p>
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
    <Card className="overflow-hidden p-0" data-testid="my-submissions">
      <h3 className="border-b border-border px-4 py-3 font-display text-sm font-semibold">
        My submissions
      </h3>
      {submissions.length === 0 ? (
        <p className="px-4 py-5 text-center text-sm text-faint">Nothing submitted yet.</p>
      ) : (
        <ul className="divide-y divide-border">
          {submissions.map((s) => (
            <li key={s.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
              <span className="font-mono text-xs text-faint">P{s.problem_ord}</span>
              <span className="text-muted">{s.language}</span>
              <span className="font-mono text-xs text-faint">
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
