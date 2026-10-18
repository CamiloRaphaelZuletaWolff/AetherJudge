"use client";

// Non-blocking cold-start notice. Appears only when the API has not answered
// /healthz within ~2 s (a sleeping Render free instance), and slides away as
// soon as it is healthy. Purely informational: it never intercepts clicks and
// the app stays fully usable underneath — requests made meanwhile simply
// complete once the server is up.
import { AnimatePresence, motion } from "framer-motion";

import { Spinner } from "@/components/ui/spinner";
import { useServerWake, type ServerWakeOptions } from "@/hooks/use-server-wake";
import { cn } from "@/lib/cn";
import { easeMove } from "@/lib/motion";

const MESSAGES = {
  waking: "Despertando el servidor (plan gratuito de Render, ~1 min)…",
  unreachable: "El servidor está tardando más de lo normal. Seguimos intentando…",
} as const;

export function ServerWakeBanner(props: ServerWakeOptions) {
  const state = useServerWake(props);
  const visible = state === "waking" || state === "unreachable";

  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed inset-x-0 bottom-4 z-[90] flex justify-center px-4 sm:justify-start"
    >
      <AnimatePresence>
        {visible && (
          <motion.div
            key="server-wake"
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 12 }}
            transition={{ duration: 0.22, ease: easeMove }}
            data-testid="server-wake-banner"
            className={cn(
              "flex max-w-md items-center gap-2.5 rounded-[var(--radius)] border bg-surface px-3.5 py-2.5 text-sm text-foreground shadow-xl",
              state === "unreachable" ? "border-v-tle/40" : "border-border",
            )}
          >
            <Spinner
              className={cn(
                "size-4 shrink-0",
                state === "unreachable" ? "text-v-tle" : "text-primary",
              )}
            />
            <span>{MESSAGES[state]}</span>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
