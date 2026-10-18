import Link from "next/link";

import { cn } from "@/lib/cn";

// Aether Judge wordmark: a monogram tile + the name. Used in the nav and on auth.
export function Logo({ href = "/dashboard", className }: { href?: string; className?: string }) {
  return (
    <Link
      href={href}
      className={cn("group inline-flex items-center gap-2.5 outline-none", className)}
      aria-label="Aether Judge home"
    >
      <span className="grid size-8 shrink-0 place-items-center rounded-[var(--radius)] bg-primary font-display text-sm font-bold text-primary-foreground shadow-sm">
        AJ
      </span>
      <span className="whitespace-nowrap font-display text-lg font-semibold tracking-tight">
        Aether Judge
      </span>
    </Link>
  );
}
