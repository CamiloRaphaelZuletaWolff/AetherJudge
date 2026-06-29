"use client";

// Tiny toast system: a context provider + useToast() hook. Used for ephemeral
// feedback (e.g. "Profile saved"). Toasts auto-dismiss and stack bottom-right.
import { AnimatePresence, motion } from "framer-motion";
import { Check, Info, X } from "lucide-react";
import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

import { cn } from "@/lib/cn";
import { easeMove } from "@/lib/motion";

type ToastTone = "success" | "info" | "error";
interface Toast {
  id: number;
  message: string;
  tone: ToastTone;
}

interface ToastContextValue {
  toast: (message: string, tone?: ToastTone) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);
let nextId = 1;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const dismiss = useCallback((id: number) => {
    setToasts((t) => t.filter((x) => x.id !== id));
  }, []);

  const toast = useCallback(
    (message: string, tone: ToastTone = "success") => {
      const id = nextId++;
      setToasts((t) => [...t, { id, message, tone }]);
      setTimeout(() => dismiss(id), 3200);
    },
    [dismiss],
  );

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="pointer-events-none fixed bottom-4 right-4 z-[100] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2">
        <AnimatePresence initial={false}>
          {toasts.map((t) => (
            <motion.div
              key={t.id}
              layout
              initial={{ opacity: 0, x: 24, scale: 0.96 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: 24, scale: 0.96 }}
              transition={{ duration: 0.22, ease: easeMove }}
              className="pointer-events-auto flex items-start gap-2.5 rounded-[var(--radius)] border border-border bg-surface p-3 shadow-xl"
            >
              <span
                className={cn(
                  "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full",
                  t.tone === "success" && "bg-v-accepted/15 text-v-accepted",
                  t.tone === "info" && "bg-v-ce/15 text-v-ce",
                  t.tone === "error" && "bg-v-wrong/15 text-v-wrong",
                )}
              >
                {t.tone === "error" ? (
                  <X className="size-3.5" />
                ) : t.tone === "info" ? (
                  <Info className="size-3.5" />
                ) : (
                  <Check className="size-3.5" />
                )}
              </span>
              <p className="flex-1 text-sm text-foreground">{t.message}</p>
              <button
                type="button"
                onClick={() => dismiss(t.id)}
                aria-label="Dismiss"
                className="cursor-pointer text-faint transition-colors hover:text-foreground"
              >
                <X className="size-4" />
              </button>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}
