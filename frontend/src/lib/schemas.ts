// Zod schemas for every API response and WebSocket event the frontend
// consumes. Parsing happens at the boundary (lib/api.ts and the WS hook), so
// contract drift between backend and frontend fails loudly at the edge
// instead of surfacing as `undefined` deep inside a component.
import { z } from "zod";

export const languageSchema = z.enum(["cpp", "python", "go"]);
export type Language = z.infer<typeof languageSchema>;

export const userSchema = z.object({
  id: z.string().uuid(),
  username: z.string(),
  email: z.string(),
  created_at: z.string(),
});
export type User = z.infer<typeof userSchema>;

export const authResponseSchema = z.object({
  user: userSchema,
  access_token: z.string().min(1),
});
export type AuthResponse = z.infer<typeof authResponseSchema>;

export const refreshResponseSchema = z.object({
  access_token: z.string().min(1),
});

export const contestSchema = z.object({
  id: z.string().uuid(),
  slug: z.string(),
  title: z.string(),
  description: z.string(),
  starts_at: z.string(),
  ends_at: z.string(),
});
export type Contest = z.infer<typeof contestSchema>;

export const contestListSchema = z.object({
  contests: z
    .array(contestSchema)
    .nullable()
    .transform((v) => v ?? []),
});

export const problemSummarySchema = z.object({
  ord: z.number().int(),
  title: z.string(),
});
export type ProblemSummary = z.infer<typeof problemSummarySchema>;

export const contestDetailSchema = z.object({
  contest: contestSchema,
  problems: z
    .array(problemSummarySchema)
    .nullable()
    .transform((v) => v ?? []),
});
export type ContestDetail = z.infer<typeof contestDetailSchema>;

export const problemSchema = z.object({
  ord: z.number().int(),
  title: z.string(),
  statement_md: z.string(),
  time_limit_ms: z.number().int(),
  memory_limit_mb: z.number().int(),
});
export type Problem = z.infer<typeof problemSchema>;

export const leaderboardRowSchema = z.object({
  rank: z.number().int(),
  username: z.string(),
  solved: z.number().int(),
  penalty_s: z.number().int(),
});
export type LeaderboardRow = z.infer<typeof leaderboardRowSchema>;

export const leaderboardSchema = z.object({
  entries: z
    .array(leaderboardRowSchema)
    .nullable()
    .transform((v) => v ?? []),
});
export type Leaderboard = z.infer<typeof leaderboardSchema>;

export const verdictSchema = z.enum([
  "accepted",
  "wrong_answer",
  "runtime_error",
  "compilation_error",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "internal_error",
]);
export type Verdict = z.infer<typeof verdictSchema>;

export const submissionSchema = z.object({
  id: z.string().uuid(),
  problem_ord: z.number().int().optional(),
  language: languageSchema,
  status: z.enum(["queued", "running", "done"]),
  verdict: verdictSchema.optional(),
  time_used_ms: z.number().int().optional(),
  submitted_at: z.string(),
  judged_at: z.string().nullable().optional(),
});
export type Submission = z.infer<typeof submissionSchema>;

export const submissionListSchema = z.object({
  submissions: z
    .array(submissionSchema)
    .nullable()
    .transform((v) => v ?? []),
});

export const runResponseSchema = z.object({
  status: z.enum(["ok", "compile_error", "runtime_error", "timeout", "memory_exceeded", "error"]),
  stdout: z.string(),
  stderr: z.string(),
  compile_output: z.string().optional(),
  exit_code: z.number().int(),
  time_used_ms: z.number().int(),
});
export type RunResponse = z.infer<typeof runResponseSchema>;

// --- WebSocket events ------------------------------------------------------

export const eventEnvelopeSchema = z.object({
  type: z.string(),
  payload: z.unknown(),
});

export const submissionUpdateSchema = z.object({
  submission_id: z.string().uuid(),
  username: z.string(),
  problem_ord: z.number().int(),
  language: z.string(),
  status: z.string(),
  verdict: verdictSchema.optional(),
  time_used_ms: z.number().int().optional(),
  submitted_at: z.string(),
});
export type SubmissionUpdate = z.infer<typeof submissionUpdateSchema>;

export const leaderboardUpdateSchema = z.object({
  entries: z
    .array(leaderboardRowSchema)
    .nullable()
    .transform((v) => v ?? []),
});

export const contestEventSchema = z.object({
  kind: z.string(),
  username: z.string().optional(),
});
