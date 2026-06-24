import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import Home from "./page";

describe("Home", () => {
  it("renders the Arena heading", () => {
    render(<Home />);

    expect(screen.getByRole("heading", { level: 1, name: "Arena" })).toBeInTheDocument();
  });

  it("describes the three product pillars", () => {
    render(<Home />);

    expect(screen.getByText("Live contests")).toBeInTheDocument();
    expect(screen.getByText("In-browser editor")).toBeInTheDocument();
    expect(screen.getByText("Instant verdicts")).toBeInTheDocument();
  });
});
