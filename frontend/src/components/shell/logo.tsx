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
      <span className="grid size-8 place-items-center rounded-[var(--radius)] bg-gradient-to-br from-primary to-emerald-600 font-display text-sm font-bold text-primary-foreground shadow-sm transition-transform group-hover:scale-105">
        AJ
      </span>
      <span className="font-display text-lg font-semibold tracking-tight">Aether Judge</span>
    </Link>
  );
}
