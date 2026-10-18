"use client";

// A template re-mounts on every navigation, which lets us play a subtle
// enter animation per route. Reduced-motion users get an instant render
// (MotionConfig collapses this to opacity-only, near-instant).
import { motion, useReducedMotion } from "framer-motion";
import { useState } from "react";

import { pageTransition } from "@/lib/motion";

export default function Template({ children }: { children: React.ReactNode }) {
  const reduced = useReducedMotion();
  const [keyboard] = useState(
    () =>
      typeof document !== "undefined" &&
      document.documentElement.dataset.inputModality === "keyboard",
  );
  return (
    <motion.div
      variants={pageTransition}
      initial={keyboard || reduced ? false : "hidden"}
      animate="show"
    >
      {children}
    </motion.div>
  );
}
