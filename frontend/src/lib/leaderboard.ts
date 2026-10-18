import type { LeaderboardRow } from "@/lib/schemas";

export interface RankChange {
  username: string;
  from: number | null;
  to: number;
  gained: number;
  solved: boolean;
}

// Rank is authoritative: never reconstruct the contest's tie-breaking rules in the UI.
export function rankChanges(before: LeaderboardRow[], after: LeaderboardRow[]): RankChange[] {
  const previous = new Map(before.map((row) => [row.username, row]));
  return after.flatMap((row) => {
    const old = previous.get(row.username);
    const gained = old ? old.rank - row.rank : 0;
    const solved = row.solved > (old?.solved ?? 0);
    return gained > 0 || solved
      ? [{ username: row.username, from: old?.rank ?? null, to: row.rank, gained, solved }]
      : [];
  });
}

export function sameStandings(a: LeaderboardRow[], b: LeaderboardRow[]): boolean {
  return (
    a.length === b.length &&
    a.every((row, i) => {
      const next = b[i];
      return (
        !!next &&
        row.username === next.username &&
        row.rank === next.rank &&
        row.solved === next.solved &&
        row.penalty_s === next.penalty_s
      );
    })
  );
}
