"use client";

// AuthGate is a UX guard, not a security boundary — the API enforces real
// authorization. It renders a restoring state while the session bootstraps
// (silent refresh on reload) and redirects guests to /auth.
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

import { Spinner } from "@/components/ui/ui";
import { useAuthStore } from "@/stores/auth";

export function AuthGate({ children }: { children: React.ReactNode }) {
  const status = useAuthStore((s) => s.status);
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (status === "guest") {
      router.replace(`/auth?next=${encodeURIComponent(pathname)}`);
    }
  }, [status, router, pathname]);

  if (status !== "authenticated") {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <div className="flex items-center gap-3 text-zinc-400">
          <Spinner />
          <span className="text-sm">Restoring session…</span>
        </div>
      </main>
    );
  }

  return <>{children}</>;
}
