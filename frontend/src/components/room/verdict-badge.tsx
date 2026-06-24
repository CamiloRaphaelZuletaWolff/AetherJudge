import { Badge, Spinner } from "@/components/ui/ui";
import type { Submission } from "@/lib/schemas";

const verdictDisplay: Record<string, { label: string; tone: Parameters<typeof Badge>[0]["tone"] }> =
  {
    accepted: { label: "Accepted", tone: "green" },
    wrong_answer: { label: "Wrong answer", tone: "red" },
    time_limit_exceeded: { label: "Time limit", tone: "amber" },
    memory_limit_exceeded: { label: "Memory limit", tone: "purple" },
    runtime_error: { label: "Runtime error", tone: "orange" },
    compilation_error: { label: "Compile error", tone: "sky" },
    internal_error: { label: "Judge error — retry", tone: "zinc" },
  };

export function VerdictBadge({ submission }: { submission: Submission }) {
  if (submission.status !== "done") {
    return (
      <Badge tone="zinc" data-testid="verdict-badge">
        <span className="flex items-center gap-1.5">
          <Spinner className="size-3" />
          {submission.status === "queued" ? "Queued" : "Judging…"}
        </span>
      </Badge>
    );
  }

  const display = verdictDisplay[submission.verdict ?? ""] ?? {
    label: submission.verdict ?? "unknown",
    tone: "zinc" as const,
  };
  return (
    <Badge tone={display.tone} data-testid="verdict-badge" data-verdict={submission.verdict}>
      {display.label}
    </Badge>
  );
}
