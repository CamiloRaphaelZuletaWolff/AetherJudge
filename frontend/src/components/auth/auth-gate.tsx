"use client";

// AuthGate is a UX guard, not a security boundary — the API enforces real
// authorization. It renders a restoring state while the session bootstraps
// (silent refresh on reload), redirects guests to /auth, and — when a route
// declares an RBAC requirement — shows a friendly "no access" screen for
// signed-in users who lack it.
import { ShieldAlert } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { usePermissions } from "@/hooks/use-permissions";
import type { Permission, Role } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth";

export function AuthGate({
  children,
  requirePermission,
  requireRole,
}: {
  children: React.ReactNode;
  requirePermission?: Permission;
  requireRole?: Role;
}) {
  const status = useAuthStore((s) => s.status);
  const router = useRouter();
  const pathname = usePathname();
  const { can, atLeast } = usePermissions();

  useEffect(() => {
    if (status === "guest") {
      router.replace(`/auth?next=${encodeURIComponent(pathname)}`);
    }
  }, [status, router, pathname]);

  if (status !== "authenticated") {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <div className="flex items-center gap-3 text-muted">
          <Spinner />
          <span className="text-sm">Restoring session…</span>
        </div>
      </main>
    );
  }

  const allowed =
    (requirePermission ? can(requirePermission) : true) &&
    (requireRole ? atLeast(requireRole) : true);

  if (!allowed) {
    return (
      <main className="flex min-h-screen items-center justify-center px-6">
        <div className="flex max-w-sm flex-col items-center gap-4 text-center">
          <div className="flex size-12 items-center justify-center rounded-full bg-danger/10 text-danger">
            <ShieldAlert className="size-6" />
          </div>
          <div className="space-y-1">
            <h1 className="font-display text-lg font-semibold">Access restricted</h1>
            <p className="text-sm text-muted">
              You don&apos;t have permission to view this page. If you think this is a mistake,
              contact an administrator.
            </p>
          </div>
          <Button variant="secondary" onClick={() => router.replace("/dashboard")}>
            Back to dashboard
          </Button>
        </div>
      </main>
    );
  }

  return <>{children}</>;
}
