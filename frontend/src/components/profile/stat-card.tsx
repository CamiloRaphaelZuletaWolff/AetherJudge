import { type LucideIcon } from "lucide-react";

import { Card } from "@/components/ui/card";

export function StatCard({
  icon: Icon,
  label,
  value,
  hint,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <Card className="flex items-center gap-3 p-4">
      <span className="grid size-10 shrink-0 place-items-center rounded-[var(--radius)] bg-primary/10 text-primary">
        <Icon className="size-5" />
      </span>
      <div className="min-w-0">
        <p className="font-display text-xl font-semibold leading-none">{value}</p>
        <p className="mt-1 truncate text-xs text-faint">{hint ?? label}</p>
      </div>
    </Card>
  );
}
