"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Field, Textarea } from "@/components/ui/input";
import { Markdown } from "@/components/ui/markdown";
import { useToast } from "@/components/ui/toast";
import { ApiError, apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { adminProblemSchema, type AdminProblem } from "@/lib/schemas";

const schema = z.object({
  title: z.string().trim().min(1, "title is required").max(200, "at most 200 characters"),
  statement_md: z.string().trim().min(1, "statement is required"),
  time_limit_ms: z.number().int("whole number").min(100, "min 100 ms").max(10000, "max 10000 ms"),
  memory_limit_mb: z.number().int("whole number").min(16, "min 16 MB").max(512, "max 512 MB"),
});
type Values = z.infer<typeof schema>;

const defaults: Values = { title: "", statement_md: "", time_limit_ms: 2000, memory_limit_mb: 128 };

export function ProblemForm({
  contestId,
  onCreated,
}: {
  contestId: string;
  onCreated?: (problem: AdminProblem) => void;
}) {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: defaults });
  // useWatch (subscription hook) rather than form.watch() — compiler-friendly,
  // and re-renders the live preview on each keystroke.
  const statement = useWatch({ control: form.control, name: "statement_md" });

  const mutation = useMutation({
    mutationFn: (v: Values) =>
      apiFetch(`/api/v1/admin/contests/${contestId}/problems`, {
        method: "POST",
        body: v,
        schema: adminProblemSchema,
        auth: true,
      }),
    onSuccess: (problem) => {
      toast(`Added problem ${problem.ord}: ${problem.title}`);
      form.reset(defaults);
      void queryClient.invalidateQueries({ queryKey: queryKeys.adminContestProblems(contestId) });
      onCreated?.(problem);
    },
    onError: (err) =>
      form.setError("root", {
        message: err instanceof ApiError ? err.message : "could not add problem",
      }),
  });

  return (
    <form
      onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
      className="flex flex-col gap-4"
      noValidate
    >
      <Field
        label="Problem title"
        placeholder="Two Sum"
        error={form.formState.errors.title?.message}
        {...form.register("title")}
      />

      <div className="flex flex-col gap-1.5">
        <label htmlFor="statement" className="text-sm font-medium text-foreground">
          Statement (Markdown)
        </label>
        <div className="grid gap-3 lg:grid-cols-2">
          <Textarea
            id="statement"
            rows={12}
            placeholder={
              "Describe the task.\n\n**Input**\n\n...\n\n**Output**\n\n...\n\n**Example**\n\nInput: `1 2` → Output: `3`"
            }
            {...form.register("statement_md")}
          />
          <div className="min-h-[12rem] overflow-auto rounded-[var(--radius)] border border-border bg-surface-2 p-3">
            {statement.trim() ? (
              <Markdown>{statement}</Markdown>
            ) : (
              <p className="text-sm text-faint">Live preview appears here.</p>
            )}
          </div>
        </div>
        {form.formState.errors.statement_md && (
          <p className="text-xs text-danger">{form.formState.errors.statement_md.message}</p>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          label="Time limit (ms)"
          type="number"
          error={form.formState.errors.time_limit_ms?.message}
          {...form.register("time_limit_ms", { valueAsNumber: true })}
        />
        <Field
          label="Memory limit (MB)"
          type="number"
          error={form.formState.errors.memory_limit_mb?.message}
          {...form.register("memory_limit_mb", { valueAsNumber: true })}
        />
      </div>

      {form.formState.errors.root && (
        <p className="rounded-[var(--radius)] border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger">
          {form.formState.errors.root.message}
        </p>
      )}
      <Button
        type="submit"
        loading={mutation.isPending}
        data-testid="add-problem-submit"
        className="self-start"
      >
        Add problem
      </Button>
    </form>
  );
}
