"use client";

import { Moon, Sun } from "lucide-react";

import { useTheme } from "@/components/theme/theme-provider";
import { cn } from "@/lib/cn";

// Icon visibility is CSS-driven off the `.dark` class on <html>, so server and
// client render identical markup (no hydration mismatch, no mount flag). Only
// the click handler needs the theme context.
export function ThemeToggle({ className }: { className?: string }) {
  const { toggle } = useTheme();

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label="Toggle color theme"
      className={cn(
        "grid size-9 cursor-pointer place-items-center rounded-[var(--radius)] border border-border-strong text-muted transition-colors hover:text-foreground",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        className,
      )}
    >
      <Sun className="size-[18px] transition-transform dark:hidden" />
      <Moon className="hidden size-[18px] transition-transform dark:block" />
    </button>
  );
}
