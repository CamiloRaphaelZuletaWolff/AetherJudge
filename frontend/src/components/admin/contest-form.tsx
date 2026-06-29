"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, apiFetch } from "@/lib/api";
import { contestSchema, type Contest } from "@/lib/schemas";
import { slugify } from "@/lib/slug";

// Mirrors the backend validators (internal/api/validate.go) so errors surface
// before the round-trip; the server stays authoritative.
const slugRe = /^[a-z0-9][a-z0-9-]{2,63}$/;
const schema = z
  .object({
    title: z.string().trim().min(1, "title is required").max(200, "at most 200 characters"),
    slug: z.string().regex(slugRe, "3–64 chars: lowercase letters, digits, hyphens"),
    description: z.string().max(2000, "at most 2000 characters"),
    starts_at: z.string().min(1, "start time is required"),
    ends_at: z.string().min(1, "end time is required"),
  })
  .refine((v) => new Date(v.ends_at) > new Date(v.starts_at), {
    message: "the end must be after the start",
    path: ["ends_at"],
  });
type Values = z.infer<typeof schema>;

// datetime-local <input> works in the browser's local zone; convert to/from ISO
// at the API boundary.
function toLocalInput(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function ContestForm({
  contest,
  onSaved,
}: {
  contest?: Contest;
  onSaved?: (contest: Contest) => void;
}) {
  const isEdit = !!contest;
  const router = useRouter();
  const { toast } = useToast();
  // In edit mode the slug is fixed; in create mode it auto-fills from the title
  // until the admin edits it directly.
  const [slugTouched, setSlugTouched] = useState(isEdit);

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: contest
      ? {
          title: contest.title,
          slug: contest.slug,
          description: contest.description,
          starts_at: toLocalInput(contest.starts_at),
          ends_at: toLocalInput(contest.ends_at),
        }
      : { title: "", slug: "", description: "", starts_at: "", ends_at: "" },
  });

  const mutation = useMutation({
    mutationFn: (values: Values) => {
      const body = {
        title: values.title,
        description: values.description,
        starts_at: new Date(values.starts_at).toISOString(),
        ends_at: new Date(values.ends_at).toISOString(),
      };
      if (isEdit) {
        return apiFetch(`/api/v1/admin/contests/${contest.id}`, {
          method: "PATCH",
          body,
          schema: contestSchema,
          auth: true,
        });
      }
      return apiFetch("/api/v1/admin/contests", {
        method: "POST",
        body: { ...body, slug: values.slug },
        schema: contestSchema,
        auth: true,
      });
    },
    onSuccess: (saved) => {
      if (isEdit) {
        toast("Contest updated");
        onSaved?.(saved);
      } else {
        toast("Contest created");
        router.push(`/admin/contests/${saved.id}`);
      }
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === "slug_taken") {
        form.setError("slug", { message: err.message });
      } else {
        form.setError("root", {
          message: err instanceof ApiError ? err.message : "could not save contest",
        });
      }
    },
  });

  return (
    <Card className="p-5">
      <form
        onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
        className="flex flex-col gap-4"
        noValidate
      >
        <Field
          label="Title"
          placeholder="Weekly Cup #1"
          error={form.formState.errors.title?.message}
          {...form.register("title", {
            onChange: (e) => {
              if (!slugTouched) {
                form.setValue("slug", slugify(e.target.value), { shouldValidate: true });
              }
            },
          })}
        />
        <Field
          label="Slug"
          disabled={isEdit}
          hint={
            isEdit
              ? "The slug can't be changed after creation."
              : "Used in URLs; auto-filled from the title."
          }
          error={form.formState.errors.slug?.message}
          {...form.register("slug", { onChange: () => setSlugTouched(true) })}
        />
        <div className="flex flex-col gap-1.5">
          <label htmlFor="contest-description" className="text-sm font-medium text-foreground">
            Description
          </label>
          <Textarea
            id="contest-description"
            rows={3}
            className="font-sans"
            placeholder="A short summary shown on the dashboard."
            {...form.register("description")}
          />
          {form.formState.errors.description && (
            <p className="text-xs text-danger">{form.formState.errors.description.message}</p>
          )}
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label="Starts"
            type="datetime-local"
            error={form.formState.errors.starts_at?.message}
            {...form.register("starts_at")}
          />
          <Field
            label="Ends"
            type="datetime-local"
            error={form.formState.errors.ends_at?.message}
            {...form.register("ends_at")}
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
          data-testid="save-contest"
          className="self-start"
        >
          {isEdit ? "Save changes" : "Create contest"}
        </Button>
      </form>
    </Card>
  );
}
