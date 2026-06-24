"use client";

import { useEffect } from "react";

import { Button } from "@/components/ui/ui";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("route error boundary:", error);
  }, [error]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 px-6">
      <h1 className="text-2xl font-bold">Something went wrong</h1>
      <p className="max-w-md text-center text-sm text-zinc-400">
        An unexpected error occurred. You can try again — if it keeps happening, the backend may be
        unreachable.
      </p>
      <Button onClick={reset}>Try again</Button>
    </main>
  );
}
