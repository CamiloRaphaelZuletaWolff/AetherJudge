"use client";

// A small, dependency-free dropdown menu: outside-click + Escape to close, and
// a spring-y entrance that emerges from the trigger corner. Used for the user
// menu in the app shell.
import { AnimatePresence, motion } from "framer-motion";
import Link from "next/link";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";

import { cn } from "@/lib/cn";
import { easeEnter } from "@/lib/motion";

export function DropdownMenu({
  trigger,
  children,
  align = "end",
  className,
  label = "Open menu",
}: {
  trigger: ReactNode;
  children: ReactNode;
  align?: "start" | "end";
  className?: string;
  label?: string;
}) {
  const [open, setOpen] = useState(false);
  const [keyboard, setKeyboard] = useState(false);
  const menuId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  useEffect(() => {
    if (open && keyboard) menuRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
  }, [open, keyboard]);

  return (
    <div ref={ref} className="relative">
      <button
        ref={triggerRef}
        type="button"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        onClick={(event) => {
          setKeyboard(event.detail === 0);
          setOpen((v) => !v);
        }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown") {
            event.preventDefault();
            setKeyboard(true);
            setOpen(true);
          }
        }}
        className="grid min-h-10 min-w-10 cursor-pointer place-items-center rounded-full outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      >
        {trigger}
      </button>
      <AnimatePresence>
        {open && (
          <motion.div
            id={menuId}
            ref={menuRef}
            role="menu"
            aria-label={label}
            initial={{ opacity: 0, scale: 0.95, y: -6 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: -4 }}
            transition={{ duration: keyboard ? 0 : 0.16, ease: easeEnter }}
            style={{ transformOrigin: align === "end" ? "top right" : "top left" }}
            onClick={() => setOpen(false)}
            onKeyDown={(event) => {
              if (event.key === "Tab") {
                setOpen(false);
                return;
              }
              const items = Array.from(
                event.currentTarget.querySelectorAll<HTMLElement>('[role="menuitem"]'),
              );
              const current = items.indexOf(document.activeElement as HTMLElement);
              let next = current;
              if (event.key === "ArrowDown") next = (current + 1) % items.length;
              else if (event.key === "ArrowUp") next = (current - 1 + items.length) % items.length;
              else if (event.key === "Home") next = 0;
              else if (event.key === "End") next = items.length - 1;
              else return;
              event.preventDefault();
              items[next]?.focus();
            }}
            onBlur={(event) => {
              if (
                !event.currentTarget.contains(event.relatedTarget) &&
                !triggerRef.current?.contains(event.relatedTarget)
              )
                setOpen(false);
            }}
            className={cn(
              "absolute z-50 mt-2 min-w-52 overflow-hidden rounded-[var(--radius)] border border-border bg-surface p-1 shadow-xl",
              align === "end" ? "right-0" : "left-0",
              className,
            )}
          >
            {children}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

const itemClass =
  "flex w-full cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm text-foreground transition-colors hover:bg-foreground/5";

export function DropdownItem({
  children,
  onClick,
  className,
}: {
  children: ReactNode;
  onClick?: () => void;
  className?: string;
}) {
  return (
    <button type="button" role="menuitem" onClick={onClick} className={cn(itemClass, className)}>
      {children}
    </button>
  );
}

export function DropdownLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link href={href} role="menuitem" className={itemClass}>
      {children}
    </Link>
  );
}

export function DropdownLabel({ children }: { children: ReactNode }) {
  return <div className="px-2.5 py-2 text-xs text-faint">{children}</div>;
}

export function DropdownSeparator() {
  return <div className="my-1 h-px bg-border" />;
}
