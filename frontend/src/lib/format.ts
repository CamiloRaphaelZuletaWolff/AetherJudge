// Small display helpers shared across pages.

// formatPenalty renders penalty seconds as h:mm:ss (or m:ss under an hour).
export function formatPenalty(totalSeconds: number): string {
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = totalSeconds % 60;
  const mm = String(m).padStart(2, "0");
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${m}:${ss}`;
}

// formatCountdown renders a duration in ms as a compact countdown.
export function formatCountdown(ms: number): string {
  if (ms <= 0) return "0:00";
  const total = Math.floor(ms / 1000);
  const d = Math.floor(total / 86400);
  const h = Math.floor((total % 86400) / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return `${m}:${String(s).padStart(2, "0")}`;
}

export type ContestPhase = "upcoming" | "active" | "past";

export function contestPhase(startsAt: string, endsAt: string, now = Date.now()): ContestPhase {
  if (now < Date.parse(startsAt)) return "upcoming";
  if (now < Date.parse(endsAt)) return "active";
  return "past";
}
