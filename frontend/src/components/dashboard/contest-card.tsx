"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { Badge, Button, Card } from "@/components/ui/ui";
import { contestPhase, formatCountdown } from "@/lib/format";
import type { Contest } from "@/lib/schemas";

export function ContestCard({ contest }: { contest: Contest }) {
  const phase = contestPhase(contest.starts_at, contest.ends_at);

  // Tick once a second so countdowns stay honest.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  return (
    <Card className="flex flex-col gap-3" data-testid="contest-card">
      <div className="flex items-start justify-between gap-2">
        <h3 className="font-semibold">{contest.title}</h3>
        {phase === "active" && <Badge tone="green">live</Badge>}
        {phase === "upcoming" && <Badge tone="sky">upcoming</Badge>}
        {phase === "past" && <Badge tone="zinc">finished</Badge>}
      </div>

      {contest.description && (
        <p className="line-clamp-2 text-sm text-zinc-400">{contest.description}</p>
      )}

      <p className="font-mono text-xs text-zinc-500">
        {phase === "active" && <>ends in {formatCountdown(Date.parse(contest.ends_at) - now)}</>}
        {phase === "upcoming" && (
          <>starts in {formatCountdown(Date.parse(contest.starts_at) - now)}</>
        )}
        {phase === "past" && <>ended {new Date(contest.ends_at).toLocaleString()}</>}
      </p>

      <div className="mt-auto">
        {phase === "active" && (
          <Link href={`/room/${contest.id}`}>
            <Button className="w-full" data-testid="enter-room">
              Enter room
            </Button>
          </Link>
        )}
        {phase === "upcoming" && (
          <Button className="w-full" variant="secondary" disabled>
            Not started yet
          </Button>
        )}
        {phase === "past" && (
          <Link href={`/room/${contest.id}`}>
            <Button className="w-full" variant="secondary">
              View results
            </Button>
          </Link>
        )}
      </div>
    </Card>
  );
}
