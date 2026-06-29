"use client";

import { motion } from "framer-motion";

import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { verdictReveal } from "@/lib/motion";
import type { Submission } from "@/lib/schemas";

// Labels are intentionally stable — the unit test and the product copy both
// depend on these exact strings.
const verdictDisplay: Record<string, { label: string; variant: BadgeVariant }> = {
  accepted: { label: "Accepted", variant: "success" },
  wrong_answer: { label: "Wrong answer", variant: "danger" },
  time_limit_exceeded: { label: "Time limit", variant: "warning" },
  memory_limit_exceeded: { label: "Memory limit", variant: "violet" },
  runtime_error: { label: "Runtime error", variant: "orange" },
  compilation_error: { label: "Compile error", variant: "info" },
  internal_error: { label: "Judge error — retry", variant: "neutral" },
};

export function VerdictBadge({ submission }: { submission: Submission }) {
  if (submission.status !== "done") {
    return (
      <Badge variant="neutral" data-testid="verdict-badge">
        <span className="flex items-center gap-1.5">
          <Spinner className="size-3" />
          {submission.status === "queued" ? "Queued" : "Judging…"}
        </span>
      </Badge>
    );
  }

  const display = verdictDisplay[submission.verdict ?? ""] ?? {
    label: submission.verdict ?? "unknown",
    variant: "neutral" as const,
  };

  return (
    <motion.span variants={verdictReveal} initial="hidden" animate="show" className="inline-flex">
      <Badge
        variant={display.variant}
        data-testid="verdict-badge"
        data-verdict={submission.verdict}
      >
        {display.label}
      </Badge>
    </motion.span>
  );
}
