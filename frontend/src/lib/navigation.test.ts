import { describe, expect, it } from "vitest";

import { safeReturnPath } from "./navigation";

describe("safeReturnPath", () => {
  it("keeps same-origin paths", () => {
    expect(safeReturnPath("/room/abc")).toBe("/room/abc");
    expect(safeReturnPath("/admin/contests/1?tab=problems#x")).toBe(
      "/admin/contests/1?tab=problems#x",
    );
  });

  it("falls back for missing or relative values", () => {
    expect(safeReturnPath(null)).toBe("/dashboard");
    expect(safeReturnPath("")).toBe("/dashboard");
    expect(safeReturnPath("room/abc")).toBe("/dashboard");
  });

  it.each([
    "//evil.example",
    "/\\evil.example",
    "/\t/evil.example",
    "/\n/evil.example",
    "/ /evil.example",
    "https://evil.example",
    "javascript:alert(1)",
    "/\u0000/evil.example",
  ])("rejects off-site target %j", (target) => {
    expect(safeReturnPath(target)).toBe("/dashboard");
  });
});
