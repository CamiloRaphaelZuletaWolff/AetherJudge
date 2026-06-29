// Shared motion language. One place for easing curves, durations, and the
// variants reused across pages so motion feels consistent everywhere.
//
// Accessibility: components render inside <MotionConfig reducedMotion="user">
// (see providers.tsx), so Framer Motion automatically collapses transform/
// layout animation to opacity-only when the user prefers reduced motion.
import type { Transition, Variants } from "framer-motion";

// Named curves (from the ui-animation skill).
export const easeEnter = [0.22, 1, 0.36, 1] as const; // entrances, hover
export const easeMove = [0.25, 1, 0.5, 1] as const; // slides, panels, reorder

export const spring: Transition = { type: "spring", stiffness: 420, damping: 34, mass: 0.9 };

// Fade + rise: the default entrance for cards, panels, sections.
export const fadeInUp: Variants = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.32, ease: easeEnter } },
};

export const fadeIn: Variants = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { duration: 0.25, ease: easeEnter } },
};

// Stagger container: children reveal 40ms apart (total kept under ~300ms).
export const staggerContainer: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.04, delayChildren: 0.02 } },
};

// Verdict reveal: a confident scale-in for the result moment.
export const verdictReveal: Variants = {
  hidden: { opacity: 0, scale: 0.85 },
  show: { opacity: 1, scale: 1, transition: { duration: 0.28, ease: easeEnter } },
};

// Route transition (used by app/template.tsx).
export const pageTransition: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.28, ease: easeEnter } },
};
