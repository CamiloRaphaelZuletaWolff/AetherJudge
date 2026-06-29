// Role-Based Access Control — frontend authorization model.
//
// This is the UI-side seam for a future backend RBAC implementation. Today the
// backend does not send a role, so `userSchema` defaults everyone to "user"
// (see lib/schemas.ts). When the backend later adds `role` to the JWT claims
// and the /api/v1/me response, every gate below lights up automatically with
// zero further frontend changes.
//
// IMPORTANT: this is a UX convenience, NOT a security boundary. The API must
// enforce real authorization server-side — hiding a button never protects an
// endpoint.
import { ROLES, type Role, type User } from "@/lib/schemas";

export { ROLES, type Role };

// Every gated capability in the app. Add new ones here, then grant them in
// ROLE_PERMISSIONS — components and routes reference these, never raw roles.
export type Permission =
  | "contest.create"
  | "contest.edit"
  | "problem.manage"
  | "submission.viewAll"
  | "user.manage"
  | "admin.access";

const ROLE_PERMISSIONS: Record<Role, readonly Permission[]> = {
  user: [],
  moderator: ["contest.create", "contest.edit", "problem.manage", "submission.viewAll"],
  admin: [
    "contest.create",
    "contest.edit",
    "problem.manage",
    "submission.viewAll",
    "user.manage",
    "admin.access",
  ],
};

export function roleOf(user: User | null | undefined): Role {
  return user?.role ?? "user";
}

export function hasRole(user: User | null | undefined, role: Role): boolean {
  return roleOf(user) === role;
}

// Role precedence, for "at least this role" checks.
const RANK: Record<Role, number> = { user: 0, moderator: 1, admin: 2 };
export function atLeast(user: User | null | undefined, role: Role): boolean {
  return RANK[roleOf(user)] >= RANK[role];
}

export function can(user: User | null | undefined, permission: Permission): boolean {
  return ROLE_PERMISSIONS[roleOf(user)].includes(permission);
}

export const ROLE_LABELS: Record<Role, string> = {
  user: "Member",
  moderator: "Moderator",
  admin: "Administrator",
};
