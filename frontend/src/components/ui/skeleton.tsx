import { cn } from "@/lib/cn";

// Reserves layout space while async content loads (no content-jumping).
export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-md bg-foreground/10", className)} />;
}
