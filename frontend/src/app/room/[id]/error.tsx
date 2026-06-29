"use client";

import Link from "next/link";
import { useEffect } from "react";

import { Button } from "@/components/ui/button";

export default function RoomError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("room error boundary:", error);
  }, [error]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 px-6 text-center">
      <h1 className="font-display text-xl font-semibold">The room hit an error</h1>
      <p className="max-w-md text-sm text-muted">
        Your code drafts are safe (they persist locally). Try reloading the room.
      </p>
      <div className="flex gap-3">
        <Button onClick={reset}>Reload room</Button>
        <Link href="/dashboard">
          <Button variant="secondary">Back to dashboard</Button>
        </Link>
      </div>
    </main>
  );
}
