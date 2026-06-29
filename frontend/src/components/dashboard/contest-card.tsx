"use client";

import { motion } from "framer-motion";
import { ArrowRight, CalendarClock, Clock, Trophy } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { contestPhase, formatCountdown } from "@/lib/format";
import { fadeInUp, spring } from "@/lib/motion";
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
    <motion.div
      variants={fadeInUp}
      whileHover={{ y: -4 }}
      transition={spring}
      data-testid="contest-card"
      className="group flex flex-col gap-3 rounded-[calc(var(--radius)+2px)] border border-border bg-surface p-5 shadow-sm transition-colors hover:border-primary/40"
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="font-display font-semibold leading-tight">{contest.title}</h3>
        {phase === "active" && (
          <Badge variant="success">
            <span className="size-1.5 animate-pulse rounded-full bg-v-accepted" /> Live
          </Badge>
        )}
        {phase === "upcoming" && <Badge variant="info">Upcoming</Badge>}
        {phase === "past" && <Badge variant="neutral">Finished</Badge>}
      </div>

      {contest.description && (
        <p className="line-clamp-2 text-sm text-muted">{contest.description}</p>
      )}

      <p className="flex items-center gap-1.5 font-mono text-xs text-faint">
        {phase === "active" && (
          <>
            <Clock className="size-3.5" /> ends in{" "}
            {formatCountdown(Date.parse(contest.ends_at) - now)}
          </>
        )}
        {phase === "upcoming" && (
          <>
            <CalendarClock className="size-3.5" /> starts in{" "}
            {formatCountdown(Date.parse(contest.starts_at) - now)}
          </>
        )}
        {phase === "past" && (
          <>
            <Trophy className="size-3.5" /> ended {new Date(contest.ends_at).toLocaleDateString()}
          </>
        )}
      </p>

      <div className="mt-auto pt-1">
        {phase === "active" && (
          <Link href={`/room/${contest.id}`}>
            <Button className="w-full" data-testid="enter-room">
              Enter room <ArrowRight className="size-4" />
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
    </motion.div>
  );
}
