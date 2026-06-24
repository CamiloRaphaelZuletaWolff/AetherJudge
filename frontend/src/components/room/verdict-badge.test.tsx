import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { Submission } from "@/lib/schemas";

import { VerdictBadge } from "./verdict-badge";

function submission(overrides: Partial<Submission>): Submission {
  return {
    id: "11111111-1111-4111-8111-111111111111",
    language: "python",
    status: "done",
    submitted_at: new Date().toISOString(),
    ...overrides,
  };
}

describe("VerdictBadge", () => {
  it("shows pending states while judging", () => {
    render(<VerdictBadge submission={submission({ status: "queued", verdict: undefined })} />);
    expect(screen.getByText("Queued")).toBeInTheDocument();
  });

  it.each([
    ["accepted", "Accepted"],
    ["wrong_answer", "Wrong answer"],
    ["time_limit_exceeded", "Time limit"],
    ["memory_limit_exceeded", "Memory limit"],
    ["runtime_error", "Runtime error"],
    ["compilation_error", "Compile error"],
    ["internal_error", "Judge error — retry"],
  ] as const)("renders %s as %s", (verdict, label) => {
    render(<VerdictBadge submission={submission({ verdict })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });
});
