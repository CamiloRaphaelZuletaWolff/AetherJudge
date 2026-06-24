import { describe, expect, it } from "vitest";

import { contestPhase, formatCountdown, formatPenalty } from "./format";

describe("formatPenalty", () => {
  it("formats sub-hour as m:ss", () => {
    expect(formatPenalty(0)).toBe("0:00");
    expect(formatPenalty(65)).toBe("1:05");
    expect(formatPenalty(1903)).toBe("31:43");
  });

  it("formats hours as h:mm:ss", () => {
    expect(formatPenalty(3600)).toBe("1:00:00");
    expect(formatPenalty(7325)).toBe("2:02:05");
  });
});

describe("formatCountdown", () => {
  it("clamps negatives", () => {
    expect(formatCountdown(-5000)).toBe("0:00");
  });

  it("renders minutes, hours, and days", () => {
    expect(formatCountdown(90 * 1000)).toBe("1:30");
    expect(formatCountdown(2 * 3600 * 1000 + 60 * 1000)).toBe("2:01:00");
    expect(formatCountdown(3 * 86400 * 1000 + 5 * 3600 * 1000)).toBe("3d 5h");
  });
});

describe("contestPhase", () => {
  const now = Date.parse("2026-06-11T12:00:00Z");

  it("classifies upcoming, active, past", () => {
    expect(contestPhase("2026-06-11T13:00:00Z", "2026-06-11T14:00:00Z", now)).toBe("upcoming");
    expect(contestPhase("2026-06-11T11:00:00Z", "2026-06-11T13:00:00Z", now)).toBe("active");
    expect(contestPhase("2026-06-11T09:00:00Z", "2026-06-11T10:00:00Z", now)).toBe("past");
  });
});
