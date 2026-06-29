"use client";

// usePermissions reads the current user's role from the auth store and exposes
// the RBAC predicates. Components ask "can I do X?", never "is my role Y?".
import { atLeast, can, hasRole, roleOf, type Permission, type Role } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth";

export function usePermissions() {
  const user = useAuthStore((s) => s.user);
  return {
    role: roleOf(user),
    can: (permission: Permission) => can(user, permission),
    hasRole: (role: Role) => hasRole(user, role),
    atLeast: (role: Role) => atLeast(user, role),
  };
}
