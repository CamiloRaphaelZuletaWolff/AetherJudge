import { cva, type VariantProps } from "class-variance-authority";
import { type HTMLAttributes } from "react";

import { cn } from "@/lib/cn";

const badge = cva(
  "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap",
  {
    variants: {
      variant: {
        neutral: "border-v-neutral/25 bg-v-neutral/12 text-v-neutral",
        primary: "border-primary/25 bg-primary/12 text-primary",
        accent: "border-accent/30 bg-accent/12 text-accent",
        success: "border-v-accepted/25 bg-v-accepted/12 text-v-accepted",
        danger: "border-v-wrong/25 bg-v-wrong/12 text-v-wrong",
        warning: "border-v-tle/25 bg-v-tle/12 text-v-tle",
        violet: "border-v-mle/25 bg-v-mle/12 text-v-mle",
        orange: "border-v-re/25 bg-v-re/12 text-v-re",
        info: "border-v-ce/25 bg-v-ce/12 text-v-ce",
      },
    },
    defaultVariants: { variant: "neutral" },
  },
);

export type BadgeVariant = NonNullable<VariantProps<typeof badge>["variant"]>;

// Spreads rest props so data-testid and friends survive (Playwright depends on
// this — the "Card bug" from phase 4).
export function Badge({
  variant,
  className,
  ...props
}: HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badge>) {
  return <span className={cn(badge({ variant }), className)} {...props} />;
}
