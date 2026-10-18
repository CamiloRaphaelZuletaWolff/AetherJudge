"use client";

import { ArrowRight, CalendarClock, Clock, Trophy } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { buttonStyles } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { contestPhase, formatCountdown } from "@/lib/format";
import type { Contest } from "@/lib/schemas";

export function ContestCard({ contest }: { contest: Contest }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  const phase = contestPhase(contest.starts_at, contest.ends_at, now);
  const duration = Math.round(
    (Date.parse(contest.ends_at) - Date.parse(contest.starts_at)) / 60000,
  );
  return (
    <article
      data-testid="contest-card"
      className={cn(
        "contest-card group relative flex flex-col gap-4 overflow-hidden rounded-2xl border border-border bg-surface p-5",
        phase === "active" && "border-primary/30",
      )}
    >
      <div className="flex items-center justify-between">
        <span
          className={cn(
            "grid size-10 place-items-center rounded-xl bg-foreground/5 text-muted",
            phase === "active" && "bg-primary/10 text-primary",
          )}
        >
          <Trophy className="size-5" />
        </span>
        {phase === "active" ? (
          <Badge variant="success">
            <span className="size-1.5 rounded-full bg-v-accepted" />
            Live now
          </Badge>
        ) : (
          <Badge variant={phase === "upcoming" ? "info" : "neutral"}>
            {phase === "upcoming" ? "Upcoming" : "Finished"}
          </Badge>
        )}
      </div>
      <div>
        <h3 className="font-display text-lg font-semibold leading-snug">{contest.title}</h3>
        <p className="mt-2 line-clamp-2 min-h-10 text-sm leading-relaxed text-muted">
          {contest.description || "Put your problem-solving skills to the test."}
        </p>
      </div>
      <div className="mt-auto flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted">
        <span className="flex items-center gap-1.5">
          <Clock className="size-3.5" />
          {duration} min
        </span>
        <span className="flex items-center gap-1.5 font-mono tabular-nums">
          {phase === "active" ? (
            <>
              <span className="size-1.5 rounded-full bg-primary" />
              {formatCountdown(Date.parse(contest.ends_at) - now)} left
            </>
          ) : phase === "upcoming" ? (
            <>
              <CalendarClock className="size-3.5" />
              in {formatCountdown(Date.parse(contest.starts_at) - now)}
            </>
          ) : (
            new Date(contest.ends_at).toLocaleDateString()
          )}
        </span>
      </div>
      <Link
        href={`/room/${contest.id}`}
        className={cn(
          buttonStyles({ variant: phase === "active" ? "primary" : "secondary" }),
          "w-full justify-between",
        )}
        data-testid={phase === "active" ? "enter-room" : undefined}
      >
        {phase === "active" ? "Enter room" : phase === "upcoming" ? "View contest" : "View results"}
        <ArrowRight className="size-4" />
      </Link>
    </article>
  );
}
