"use client";

import { motion, useReducedMotion } from "framer-motion";
import { useId, useState } from "react";
import { cn } from "@/lib/cn";
import { easeMove } from "@/lib/motion";

export function Tabs<T extends string>({
  value,
  options,
  onChange,
  className,
  label = "View",
}: {
  value: T;
  options: ReadonlyArray<{ value: T; label: string; panelId?: string }>;
  onChange: (value: T) => void;
  className?: string;
  label?: string;
}) {
  const layoutId = useId();
  const reduced = useReducedMotion();
  const [keyboard, setKeyboard] = useState(false);
  return (
    <div
      role="tablist"
      aria-label={label}
      className={cn(
        "relative flex gap-1 overflow-x-auto rounded-xl border border-border bg-surface-2 p-1",
        className,
      )}
    >
      {options.map((opt, index) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            role="tab"
            type="button"
            aria-selected={active}
            aria-controls={opt.panelId}
            tabIndex={active ? 0 : -1}
            onClick={(event) => {
              setKeyboard(event.detail === 0);
              onChange(opt.value);
            }}
            onKeyDown={(event) => {
              let next = index;
              if (event.key === "ArrowRight") next = (index + 1) % options.length;
              else if (event.key === "ArrowLeft")
                next = (index - 1 + options.length) % options.length;
              else if (event.key === "Home") next = 0;
              else if (event.key === "End") next = options.length - 1;
              else return;
              event.preventDefault();
              setKeyboard(true);
              const option = options[next];
              if (!option) return;
              onChange(option.value);
              const buttons =
                event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>(
                  '[role="tab"]',
                );
              buttons?.[next]?.focus();
            }}
            className={cn(
              "relative min-h-9 flex-1 cursor-pointer whitespace-nowrap rounded-lg px-3 py-2 text-xs font-medium transition-colors sm:text-sm",
              active ? "text-foreground" : "text-muted hover:text-foreground",
            )}
          >
            {active && (
              <motion.span
                layoutId={layoutId}
                className="absolute inset-0 rounded-lg bg-surface shadow-sm ring-1 ring-border"
                transition={{ duration: keyboard || reduced ? 0 : 0.2, ease: easeMove }}
              />
            )}
            <span className="relative z-10">{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
