// Centralized TanStack Query keys so cache writes (WS hook) and reads
// (components) can never drift apart.
export const queryKeys = {
  contests: (filter: string) => ["contests", filter] as const,
  contest: (id: string) => ["contest", id] as const,
  problem: (contestId: string, ord: number) => ["problem", contestId, ord] as const,
  leaderboard: (contestId: string) => ["leaderboard", contestId] as const,
  mySubmissions: (contestId: string) => ["my-submissions", contestId] as const,
};
