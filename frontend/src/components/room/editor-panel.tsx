"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "framer-motion";
import { Check, Code2, Play, Send, Terminal } from "lucide-react";
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
  submissionDisabledReason,
}: {
  contestId: string;
  problemOrd: number;
  problemKey: string;
  submissionDisabledReason?: string;
}) {
  const queryClient = useQueryClient();
  const { theme } = useTheme();
  const { language, setLanguage, getDraft, setDraft } = useEditorStore();
  const code = getDraft(problemKey, language);

  const [stdin, setStdin] = useState("");
  const [runResult, setRunResult] = useState<RunResponse | null>(null);
  const [basicEditor, setBasicEditor] = useState(false);

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
      <div className="flex items-center justify-between px-1">
        <h2 className="flex items-center gap-2 font-display text-sm font-semibold">
          <Code2 className="size-4 text-primary" /> Solution{" "}
          <span className="font-mono text-xs text-muted">P{problemOrd}</span>
        </h2>
        <span className="flex items-center gap-1.5 text-[11px] text-muted">
          <Check className="size-3 text-primary" /> Draft saved on this device
        </span>
      </div>
      <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-surface p-2">
        <div className="relative">
          <select
            value={language}
            onChange={(e) => {
              setLanguage(languageSchema.parse(e.target.value));
              setRunResult(null);
              run.reset();
              submit.reset();
            }}
            data-testid="language-select"
            aria-label="Language"
            disabled={run.isPending || submit.isPending}
            className="min-h-10 cursor-pointer rounded-[var(--radius)] border border-border-strong bg-surface-2 px-3 py-1.5 text-sm font-medium text-foreground outline-none focus:border-primary"
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
            disabled={!code.trim()}
            onClick={() => {
              setRunResult(null);
              run.mutate();
            }}
            data-testid="run-button"
          >
            {!run.isPending && <Play className="size-4" />} Run
          </Button>
          <Button
            loading={submit.isPending}
            disabled={!!submissionDisabledReason || !code.trim()}
            title={submissionDisabledReason ?? "Submit against the contest test cases"}
            onClick={() => submit.mutate()}
            data-testid="submit-button"
          >
            {!submit.isPending && <Send className="size-4" />} Submit
          </Button>
        </div>
      </div>

      {submissionDisabledReason && (
        <p className="px-1 text-xs text-muted">
          {submissionDisabledReason}. You can still edit and run code.
        </p>
      )}
      <div className="overflow-hidden rounded-[calc(var(--radius)+2px)] border border-border bg-surface">
        <div className="flex items-center justify-between border-b border-border bg-surface-2 px-3 py-2 text-[11px] text-muted">
          <span className="font-mono">
            solution.{language === "python" ? "py" : language === "cpp" ? "cpp" : "go"}
          </span>
          <button
            type="button"
            onClick={() => setBasicEditor(!basicEditor)}
            className="cursor-pointer rounded px-2 py-1 text-primary hover:bg-primary/10"
          >
            {basicEditor ? "Use advanced editor" : "Use basic editor"}
          </button>
        </div>
        <div className="h-[400px]">
          {basicEditor ? (
            <textarea
              aria-label="Solution code"
              value={code}
              onChange={(event) => setDraft(problemKey, language, event.target.value)}
              spellCheck={false}
              autoCapitalize="off"
              autoComplete="off"
              className="h-full w-full resize-none bg-surface p-4 font-mono text-sm leading-relaxed text-foreground outline-none"
            />
          ) : (
            <MonacoEditor
              height="100%"
              theme={theme === "dark" ? "vs-dark" : "light"}
              language={monacoLanguage[language]}
              value={code}
              onChange={(value) => setDraft(problemKey, language, value ?? "")}
              loading={
                <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
                  <Spinner />
                  <p className="text-sm text-muted">Loading your code editor…</p>
                  <button
                    type="button"
                    onClick={() => setBasicEditor(true)}
                    className="cursor-pointer rounded-lg border border-border px-3 py-2 text-xs text-primary hover:bg-primary/5"
                  >
                    Start writing in the basic editor
                  </button>
                </div>
              }
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
          )}
        </div>
      </div>

      <Card className="flex flex-col gap-3 p-4">
        <label htmlFor="custom-stdin" className="text-xs font-medium text-muted">
          <span className="flex items-center gap-2">
            <Terminal className="size-3.5" /> Test your solution
          </span>
        </label>
        <Textarea
          id="custom-stdin"
          value={stdin}
          onChange={(e) => setStdin(e.target.value)}
          data-testid="stdin-input"
          rows={2}
          placeholder="Paste your test input here…"
        />
        <p className="text-[11px] text-muted">
          Run uses this input. Submit checks the contest&apos;s test cases and counts toward your
          score.
        </p>
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
        <p
          role="status"
          className="feedback-enter flex items-center gap-2 rounded-xl border border-primary/20 bg-primary/5 p-3 text-sm text-primary"
          data-testid="submit-confirmation"
        >
          <Check className="size-4 shrink-0" />
          Submitted — watch the verdict below.
        </p>
      )}

      {run.isError && <ErrorNotice message={run.error.message} />}
      {runResult && <RunOutput result={runResult} />}

      <SubmissionsList contestId={contestId} problemOrd={problemOrd} />
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
    <Card role="status" className="feedback-enter flex flex-col gap-2 p-4" data-testid="run-output">
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

function SubmissionsList({ contestId, problemOrd }: { contestId: string; problemOrd: number }) {
  const [onlyCurrent, setOnlyCurrent] = useState(false);
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

  const submissions = query.data.submissions.filter(
    (s) => !onlyCurrent || s.problem_ord === problemOrd,
  );
  return (
    <Card className="overflow-hidden p-0" data-testid="my-submissions">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-3">
        <h3 className="font-display text-sm font-semibold">
          My submissions{" "}
          <span className="ml-1 font-mono text-xs text-muted">{submissions.length}</span>
        </h3>
        <label className="flex cursor-pointer items-center gap-2 text-xs text-muted">
          <input
            type="checkbox"
            checked={onlyCurrent}
            onChange={(e) => setOnlyCurrent(e.target.checked)}
            className="accent-primary"
          />
          This problem
        </label>
      </div>
      {submissions.length === 0 ? (
        <div className="px-4 py-8 text-center">
          <Send className="mx-auto mb-2 size-5 text-faint" />
          <p className="text-sm font-medium">Your next attempt starts here</p>
          <p className="mt-1 text-xs text-muted">
            Submit your solution to see its verdict and execution time.
          </p>
        </div>
      ) : (
        <ul
          className="max-h-80 overflow-y-auto divide-y divide-border"
          aria-live="polite"
          aria-relevant="additions text"
        >
          <AnimatePresence initial={false}>
            {submissions.map((s) => (
              <motion.li
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.18 }}
                key={s.id}
                className="flex flex-wrap items-center gap-3 px-4 py-3 text-sm"
              >
                <span className="font-mono text-xs text-faint">P{s.problem_ord}</span>
                <span className="text-muted">{s.language}</span>
                <span className="font-mono text-xs text-faint">
                  {new Date(s.submitted_at).toLocaleTimeString()}
                </span>
                <span className="ml-auto">
                  <VerdictBadge submission={s} />
                </span>
              </motion.li>
            ))}
          </AnimatePresence>
        </ul>
      )}
    </Card>
  );
}
