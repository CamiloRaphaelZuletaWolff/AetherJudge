"use client";

import { motion } from "framer-motion";
import { ArrowRight, Gauge, Radio, ShieldCheck, Terminal } from "lucide-react";
import Link from "next/link";

import { Logo } from "@/components/shell/logo";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { fadeInUp, staggerContainer } from "@/lib/motion";

const pillars = [
  {
    icon: Radio,
    title: "Live contests",
    description: "Join a room, race the clock, and watch rankings move with every submission.",
  },
  {
    icon: Terminal,
    title: "In-browser editor",
    description: "Write C++, Python, or Go with full syntax highlighting — no local setup.",
  },
  {
    icon: Gauge,
    title: "Instant verdicts",
    description: "Submissions run in isolated sandboxes and are judged in seconds.",
  },
] as const;

export default function Home() {
  return (
    <div className="relative min-h-screen overflow-hidden">
      <div className="bg-grid pointer-events-none absolute inset-0 [mask-image:radial-gradient(ellipse_at_top,black,transparent_70%)] opacity-60" />
      <div className="pointer-events-none absolute -top-40 left-1/2 size-[40rem] -translate-x-1/2 rounded-full bg-primary/10 blur-3xl" />

      <header className="relative mx-auto flex h-16 w-full max-w-6xl items-center px-6">
        <Logo href="/" />
        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          <Link href="/auth">
            <Button variant="secondary" size="sm">
              Sign in
            </Button>
          </Link>
        </div>
      </header>

      <main className="relative mx-auto flex w-full max-w-6xl flex-col items-center px-6 pb-24 pt-16 text-center sm:pt-24">
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="show"
          className="flex flex-col items-center"
        >
          <motion.span
            variants={fadeInUp}
            className="mb-6 inline-flex items-center gap-2 rounded-full border border-border bg-surface px-3 py-1 text-xs text-muted"
          >
            <ShieldCheck className="size-3.5 text-primary" />
            Sandboxed judging · real-time leaderboards
          </motion.span>

          <motion.h1
            variants={fadeInUp}
            className="max-w-3xl font-display text-5xl font-bold tracking-tight sm:text-7xl"
          >
            Aether Judge
          </motion.h1>

          <motion.p variants={fadeInUp} className="mt-5 max-w-xl text-lg text-muted">
            A real-time competitive programming platform. Solve algorithmic problems against other
            players and watch the leaderboard update live.
          </motion.p>

          <motion.div
            variants={fadeInUp}
            className="mt-8 flex flex-wrap items-center justify-center gap-3"
          >
            <Link href="/auth">
              <Button size="lg">
                Get started <ArrowRight className="size-4" />
              </Button>
            </Link>
            <Link href="/dashboard">
              <Button variant="secondary" size="lg">
                Browse contests
              </Button>
            </Link>
          </motion.div>
        </motion.div>

        <motion.ul
          variants={staggerContainer}
          initial="hidden"
          animate="show"
          className="mt-20 grid w-full max-w-4xl gap-4 text-left sm:grid-cols-3"
        >
          {pillars.map((pillar) => {
            const Icon = pillar.icon;
            return (
              <motion.li
                key={pillar.title}
                variants={fadeInUp}
                className="rounded-[calc(var(--radius)+2px)] border border-border bg-surface p-5"
              >
                <span className="mb-3 inline-grid size-10 place-items-center rounded-[var(--radius)] bg-primary/10 text-primary">
                  <Icon className="size-5" />
                </span>
                <h2 className="font-display font-semibold">{pillar.title}</h2>
                <p className="mt-1 text-sm text-muted">{pillar.description}</p>
              </motion.li>
            );
          })}
        </motion.ul>
      </main>
    </div>
  );
}
