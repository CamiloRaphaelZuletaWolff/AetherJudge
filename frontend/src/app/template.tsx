"use client";

// A template re-mounts on every navigation, which lets us play a subtle
// enter animation per route. Reduced-motion users get an instant render
// (MotionConfig collapses this to opacity-only, near-instant).
import { motion } from "framer-motion";

import { pageTransition } from "@/lib/motion";

export default function Template({ children }: { children: React.ReactNode }) {
  return (
    <motion.div variants={pageTransition} initial="hidden" animate="show">
      {children}
    </motion.div>
  );
}
