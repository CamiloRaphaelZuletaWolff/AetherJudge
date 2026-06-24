import Link from "next/link";

const pillars = [
  {
    title: "Live contests",
    description: "Join a room, race the clock, and watch rankings move with every submission.",
  },
  {
    title: "In-browser editor",
    description: "Write C++, Python, or Go with full syntax highlighting — no local setup.",
  },
  {
    title: "Instant verdicts",
    description: "Submissions run in isolated sandboxes and are judged in seconds.",
  },
] as const;

export default function Home() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-10 px-6 py-16">
      <div className="flex flex-col items-center gap-4 text-center">
        <h1 className="text-5xl font-bold tracking-tight sm:text-7xl">Arena</h1>
        <p className="max-w-xl text-lg text-zinc-400">
          A real-time competitive programming platform. Solve algorithmic problems against other
          players and watch the leaderboard update live.
        </p>
      </div>

      <div className="flex gap-3">
        <Link
          href="/auth"
          className="rounded-md bg-emerald-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-emerald-500"
        >
          Sign in / Sign up
        </Link>
        <Link
          href="/dashboard"
          className="rounded-md border border-zinc-700 bg-zinc-900 px-5 py-2.5 text-sm font-medium text-zinc-200 transition-colors hover:bg-zinc-800"
        >
          Browse contests
        </Link>
      </div>

      <ul className="grid w-full max-w-4xl gap-4 sm:grid-cols-3">
        {pillars.map((pillar) => (
          <li key={pillar.title} className="rounded-lg border border-zinc-800 p-5">
            <h2 className="mb-2 font-semibold">{pillar.title}</h2>
            <p className="text-sm text-zinc-400">{pillar.description}</p>
          </li>
        ))}
      </ul>
    </main>
  );
}
