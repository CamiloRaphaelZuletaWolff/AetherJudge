import Link from "next/link";

import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 px-6 text-center">
      <p className="font-display text-7xl font-bold tracking-tight text-primary">404</p>
      <p className="text-sm text-muted">That page doesn&apos;t exist.</p>
      <Link href="/dashboard">
        <Button variant="secondary">Back to the dashboard</Button>
      </Link>
    </main>
  );
}
