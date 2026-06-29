// cn merges Tailwind class lists, resolving conflicts (later wins) so variant
// composition (cva) and per-call overrides combine predictably.
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
