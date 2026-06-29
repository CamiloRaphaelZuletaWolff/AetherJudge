"use client";

// Declarative RBAC gates for rendering. Hide UI the user can't act on.
// `<Can perm="contest.create">…</Can>` or `<RoleGate role="admin">…</RoleGate>`.
// Reminder: presentation only — the API enforces real authorization.
import type { ReactNode } from "react";

import { usePermissions } from "@/hooks/use-permissions";
import type { Permission, Role } from "@/lib/rbac";

export function Can({
  perm,
  role,
  fallback = null,
  children,
}: {
  perm?: Permission;
  role?: Role;
  fallback?: ReactNode;
  children: ReactNode;
}) {
  const { can, atLeast } = usePermissions();
  const allowed = (perm ? can(perm) : true) && (role ? atLeast(role) : true);
  return <>{allowed ? children : fallback}</>;
}

export function RoleGate({
  role,
  fallback = null,
  children,
}: {
  role: Role;
  fallback?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Can role={role} fallback={fallback}>
      {children}
    </Can>
  );
}
