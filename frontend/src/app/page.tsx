"use client";

import { ArrowRight, Check, Gauge, Radio, Terminal } from "lucide-react";
import Link from "next/link";
import { ScoreboardDemo } from "@/components/room/scoreboard-demo";
import { Logo } from "@/components/shell/logo";
import { buttonStyles } from "@/components/ui/button";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { cn } from "@/lib/cn";

const pillars = [
  {
    icon: Radio,
    title: "Live contests",
    description:
      "Find your room and race the clock. Every solve brings a new turn in the standings.",
    step: "01",
  },
  {
    icon: Terminal,
    title: "In-browser editor",
    description:
      "Your workspace is ready. Pick C++, Python, or Go, and put your solution to the test.",
    step: "02",
  },
  {
    icon: Gauge,
    title: "Instant verdicts",
    description:
      "Get clear feedback on every attempt. Learn, improve, and climb with your next accepted solution.",
    step: "03",
  },
] as const;

export default function Home() {
  return (
    <div className="relative min-h-screen overflow-hidden">
      <div className="bg-grid pointer-events-none absolute inset-0 opacity-35 [mask-image:linear-gradient(black,transparent_65%)]" />
      <header className="relative mx-auto flex h-20 w-full max-w-7xl items-center px-5 sm:px-8">
        <Logo href="/" />
        <nav aria-label="Public navigation" className="ml-auto flex items-center gap-3">
          <Link
            href="/dashboard"
            className="hidden text-sm text-muted transition-colors hover:text-foreground sm:block"
          >
            Explore contests
          </Link>
          <ThemeToggle />
          <Link href="/auth" className={buttonStyles({ variant: "secondary", size: "sm" })}>
            Sign in <ArrowRight className="size-3.5" />
          </Link>
        </nav>
      </header>
      <main className="relative mx-auto w-full max-w-7xl px-5 pb-16 sm:px-8">
        <section className="grid items-center gap-10 py-12 lg:grid-cols-[1fr_1fr] lg:gap-14 lg:py-20">
          <div>
            <span className="mb-6 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/5 px-3 py-1.5 text-xs font-medium text-primary">
              <span className="size-1.5 rounded-full bg-primary" />
              Built for the thrill of the solve
            </span>
            <h1 className="font-display text-5xl font-bold leading-[1.05] tracking-tight sm:text-7xl">
              Aether Judge
            </h1>
            <p className="mt-3 font-display text-3xl font-medium leading-tight tracking-tight text-muted sm:text-4xl">
              Think fast.
              <br />
              <span className="text-primary">Rise together.</span>
            </p>
            <p className="mt-6 max-w-md text-base leading-relaxed text-muted">
              Your code. Your next breakthrough. Join live programming contests and watch every
              accepted solution move the competition.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <Link href="/auth" className={buttonStyles({ size: "lg" })}>
                Get started <ArrowRight className="size-4" />
              </Link>
              <Link
                href="/dashboard"
                className={buttonStyles({ variant: "secondary", size: "lg" })}
              >
                Browse contests
              </Link>
            </div>
            <div className="mt-6 flex flex-wrap gap-4 text-xs text-muted">
              <span className="flex items-center gap-1.5">
                <Check className="size-3.5 text-primary" />
                No setup needed
              </span>
              <span className="flex items-center gap-1.5">
                <Check className="size-3.5 text-primary" />
                C++ · Python · Go
              </span>
            </div>
          </div>
          <div className="relative min-w-0">
            <div className="pointer-events-none absolute -inset-4 rounded-3xl border border-primary/10 bg-primary/[0.025]" />
            <div className="relative">
              <ScoreboardDemo />
            </div>
            <p className="mt-6 text-center text-xs text-muted">
              Sample standings. Try a solve and follow the climb.
            </p>
          </div>
        </section>
        <section className="border-t border-border pt-10">
          <div className="mb-6 flex items-end justify-between gap-4">
            <div>
              <p className="eyebrow mb-2">From idea to accepted</p>
              <h2 className="font-display text-2xl font-semibold tracking-tight">
                Less friction. More solving.
              </h2>
            </div>
            <span className="hidden font-mono text-xs text-faint sm:block">
              THE CONTEST WORKFLOW
            </span>
          </div>
          <div className="grid gap-4 sm:grid-cols-3">
            {pillars.map(({ icon: Icon, title, description, step }) => (
              <article
                key={title}
                className={cn("rounded-2xl border border-border bg-surface p-6")}
              >
                <div className="mb-5 flex items-center justify-between">
                  <span className="grid size-10 place-items-center rounded-xl bg-primary/10 text-primary">
                    <Icon className="size-5" />
                  </span>
                  <span className="font-mono text-xs text-faint">{step}</span>
                </div>
                <h3 className="font-display font-semibold">{title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted">{description}</p>
              </article>
            ))}
          </div>
        </section>
      </main>
      <footer className="relative border-t border-border">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-5 py-6 text-xs text-muted sm:px-8">
          <span>Aether Judge · A place for your next breakthrough.</span>
          <Link href="/dashboard" className="inline-flex items-center gap-1.5 text-primary">
            Find your next contest <ArrowRight className="size-3" />
          </Link>
        </div>
      </footer>
    </div>
  );
}
