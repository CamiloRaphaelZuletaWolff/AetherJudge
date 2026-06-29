"use client";

import { motion } from "framer-motion";
import { useId } from "react";

import { cn } from "@/lib/cn";
import { easeMove } from "@/lib/motion";

export function Tabs<T extends string>({
  value,
  options,
  onChange,
  className,
}: {
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  onChange: (value: T) => void;
  className?: string;
}) {
  const layoutId = useId();
  return (
    <div
      role="tablist"
      className={cn(
        "relative flex gap-1 rounded-[var(--radius)] border border-border bg-surface-2 p-1",
        className,
      )}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            role="tab"
            type="button"
            aria-selected={active}
            onClick={() => onChange(opt.value)}
            className={cn(
              "relative flex-1 cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
              active ? "text-foreground" : "text-muted hover:text-foreground",
            )}
          >
            {active && (
              <motion.span
                layoutId={layoutId}
                className="absolute inset-0 rounded-md bg-surface shadow-sm ring-1 ring-border"
                transition={{ duration: 0.2, ease: easeMove }}
              />
            )}
            <span className="relative z-10">{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
