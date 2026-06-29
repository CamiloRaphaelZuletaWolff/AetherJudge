"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileDown, Plus, Trash2, Upload } from "lucide-react";
import { useRef, useState, type ChangeEvent } from "react";

import { Button } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/card";
import { Textarea } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useToast } from "@/components/ui/toast";
import { ApiError, apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { parsedTestCasesSchema, testCaseListSchema } from "@/lib/schemas";

export interface DraftCase {
  stdin: string;
  expected_output: string;
}

const emptyCase = (): DraftCase => ({ stdin: "", expected_output: "" });

const ACCEPTED_FILES = ".txt,.md,.csv,.json,.xlsx";

// A ready-to-edit .txt template in the block format (ADR-0016).
const TEMPLATE = ["1 2", "---", "3", "===", "10 20", "---", "30", ""].join("\n");

function downloadTemplate() {
  const url = URL.createObjectURL(new Blob([TEMPLATE], { type: "text/plain" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = "test-cases-template.txt";
  a.click();
  URL.revokeObjectURL(url);
}

// TestCaseRows is a controlled, pure editor for a set of draft test cases —
// kept separate from the data-fetching container so it is testable without
// providers.
export function TestCaseRows({
  rows,
  onChange,
}: {
  rows: DraftCase[];
  onChange: (rows: DraftCase[]) => void;
}) {
  const update = (i: number, patch: Partial<DraftCase>) =>
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));

  return (
    <div className="flex flex-col gap-3">
      {rows.map((row, i) => (
        <div key={i} data-testid="tc-row" className="grid gap-2 sm:grid-cols-2">
          <div className="flex flex-col gap-1">
            <span className="text-xs text-muted">Input (stdin)</span>
            <Textarea
              rows={2}
              data-testid="tc-stdin"
              value={row.stdin}
              onChange={(e) => update(i, { stdin: e.target.value })}
            />
          </div>
          <div className="flex flex-col gap-1">
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted">Expected output</span>
              <button
                type="button"
                data-testid="tc-remove"
                aria-label={`Remove test case ${i + 1}`}
                onClick={() => onChange(rows.filter((_, idx) => idx !== i))}
                className="cursor-pointer text-faint transition-colors hover:text-danger"
              >
                <Trash2 className="size-4" />
              </button>
            </div>
            <Textarea
              rows={2}
              data-testid="tc-expected"
              value={row.expected_output}
              onChange={(e) => update(i, { expected_output: e.target.value })}
            />
          </div>
        </div>
      ))}
      <Button
        type="button"
        variant="secondary"
        size="sm"
        data-testid="add-case"
        onClick={() => onChange([...rows, emptyCase()])}
        className="self-start"
      >
        <Plus className="size-4" /> Add case
      </Button>
    </div>
  );
}

export function TestCaseEditor({ contestId, problemId }: { contestId: string; problemId: string }) {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<DraftCase[]>([emptyCase()]);

  const existing = useQuery({
    queryKey: queryKeys.adminProblemTestCases(problemId),
    queryFn: () =>
      apiFetch(`/api/v1/admin/problems/${problemId}/test-cases`, {
        schema: testCaseListSchema,
        auth: true,
      }),
  });

  // Drop blank rows; a case may legitimately have empty stdin OR expected, but
  // not both.
  const validDraft = draft.filter((c) => c.stdin !== "" || c.expected_output !== "");

  const save = useMutation({
    mutationFn: (cases: DraftCase[]) =>
      apiFetch(`/api/v1/admin/problems/${problemId}/test-cases`, {
        method: "POST",
        body: { cases },
        auth: true,
      }),
    onSuccess: () => {
      toast(`Saved ${validDraft.length} test case${validDraft.length === 1 ? "" : "s"}`);
      setDraft([emptyCase()]);
      void queryClient.invalidateQueries({ queryKey: queryKeys.adminProblemTestCases(problemId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.adminContestProblems(contestId) });
    },
    onError: (err) =>
      toast(err instanceof ApiError ? err.message : "could not save test cases", "error"),
  });

  // Bulk import: the server parses the file and returns cases (no write); we
  // drop them into the editable draft for review, then the existing Save commits
  // them through the same validated batch endpoint (ADR-0016).
  const fileRef = useRef<HTMLInputElement>(null);
  const importFile = useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData();
      fd.append("file", file);
      return apiFetch("/api/v1/admin/test-cases/parse", {
        method: "POST",
        body: fd,
        schema: parsedTestCasesSchema,
        auth: true,
      });
    },
    onSuccess: (res) => {
      setDraft((cur) => {
        const kept = cur.filter((c) => c.stdin !== "" || c.expected_output !== "");
        const merged = [
          ...kept,
          ...res.cases.map((c) => ({ stdin: c.stdin, expected_output: c.expected_output })),
        ];
        return merged.length > 0 ? merged : [emptyCase()];
      });
      toast(`Parsed ${res.count} test case${res.count === 1 ? "" : "s"} — review and save`);
    },
    onError: (err) =>
      toast(err instanceof ApiError ? err.message : "could not parse the file", "error"),
  });

  const onFile = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) importFile.mutate(file);
    e.target.value = ""; // allow re-selecting the same file
  };

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h4 className="mb-2 text-xs font-semibold tracking-wide text-muted uppercase">
          Existing test cases
        </h4>
        {existing.isPending ? (
          <div className="flex justify-center py-3">
            <Spinner className="size-4" />
          </div>
        ) : existing.isError ? (
          <ErrorNotice message={existing.error.message} onRetry={() => void existing.refetch()} />
        ) : existing.data.test_cases.length === 0 ? (
          <p className="text-sm text-faint">None yet — add the first below.</p>
        ) : (
          <ul className="divide-y divide-border rounded-[var(--radius)] border border-border">
            {existing.data.test_cases.map((tc) => (
              <li key={tc.ord} className="grid grid-cols-2 gap-3 px-3 py-2 font-mono text-xs">
                <span className="truncate text-muted">in: {tc.stdin || "∅"}</span>
                <span className="truncate text-muted">out: {tc.expected_output || "∅"}</span>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div>
        <div className="mb-2 flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <h4 className="text-xs font-semibold tracking-wide text-muted uppercase">Add cases</h4>
          <input
            ref={fileRef}
            type="file"
            accept={ACCEPTED_FILES}
            onChange={onFile}
            className="hidden"
            data-testid="import-file-input"
          />
          <Button
            type="button"
            variant="secondary"
            size="sm"
            loading={importFile.isPending}
            onClick={() => fileRef.current?.click()}
            data-testid="import-file"
          >
            {!importFile.isPending && <Upload className="size-4" />} Import from file
          </Button>
          <button
            type="button"
            onClick={downloadTemplate}
            className="inline-flex cursor-pointer items-center gap-1 text-xs text-muted transition-colors hover:text-foreground"
          >
            <FileDown className="size-3.5" /> template
          </button>
          <span className="text-xs text-faint">.txt · .md · .csv · .json · .xlsx</span>
        </div>
        <TestCaseRows rows={draft} onChange={setDraft} />
      </div>

      <Button
        onClick={() => save.mutate(validDraft)}
        loading={save.isPending}
        disabled={validDraft.length === 0}
        data-testid="save-test-cases"
        className="self-start"
      >
        Save {validDraft.length || ""} test case{validDraft.length === 1 ? "" : "s"}
      </Button>
    </div>
  );
}
