import Link from "next/link";

export default function NotFound() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 px-6">
      <h1 className="text-5xl font-bold tracking-tight">404</h1>
      <p className="text-sm text-zinc-400">That page doesn&apos;t exist.</p>
      <Link href="/dashboard" className="text-sm text-emerald-400 hover:underline">
        Back to the dashboard
      </Link>
    </main>
  );
}
