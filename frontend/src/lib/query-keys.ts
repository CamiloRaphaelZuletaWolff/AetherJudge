// Centralized TanStack Query keys so cache writes (WS hook) and reads
// (components) can never drift apart.
export const queryKeys = {
  contests: (filter: string) => ["contests", filter] as const,
  contest: (id: string) => ["contest", id] as const,
  problem: (contestId: string, ord: number) => ["problem", contestId, ord] as const,
  leaderboard: (contestId: string) => ["leaderboard", contestId] as const,
  mySubmissions: (contestId: string) => ["my-submissions", contestId] as const,
  // Admin / moderator views (RBAC; ADR-0014).
  adminUsers: () => ["admin-users"] as const,
  contestAllSubmissions: (contestId: string) => ["contest-all-submissions", contestId] as const,
  // Content authoring (ADR-0015).
  adminContestProblems: (contestId: string) => ["admin-contest-problems", contestId] as const,
  adminProblemTestCases: (problemId: string) => ["admin-problem-test-cases", problemId] as const,
};
