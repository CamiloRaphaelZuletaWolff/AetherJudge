import { Skeleton } from "@/components/ui/skeleton";

export default function DashboardLoading() {
  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="mt-3 h-4 w-80" />
      <div className="mt-10 flex flex-col gap-10">
        {[0, 1].map((s) => (
          <div key={s} className="flex flex-col gap-3">
            <Skeleton className="h-6 w-32" />
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {[0, 1, 2].map((c) => (
                <Skeleton key={c} className="h-44 w-full rounded-[calc(var(--radius)+2px)]" />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
