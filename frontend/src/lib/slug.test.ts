import { describe, expect, it } from "vitest";

import { slugify } from "./slug";

describe("slugify", () => {
  it("lowercases and hyphenates spaces", () => {
    expect(slugify("Weekly Cup")).toBe("weekly-cup");
  });

  it("collapses non-alphanumeric runs and trims ends", () => {
    expect(slugify("  Round #2 -- Finals!! ")).toBe("round-2-finals");
  });

  it("clamps to 64 characters", () => {
    expect(slugify("a".repeat(100))).toHaveLength(64);
  });

  it("never ends with a hyphen after clamping", () => {
    expect(slugify("a".repeat(63) + " bbb")).not.toMatch(/-$/);
  });
});
