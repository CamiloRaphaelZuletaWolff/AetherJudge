"use client";

// Admin user management (RBAC user.manage; ADR-0014). Lists every account and
// lets an admin promote/demote roles. The current admin's own row is locked,
// mirroring the backend's anti-lockout guardrail — but the API is the real
// boundary, not this UI.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Avatar } from "@/components/ui/avatar";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Card, ErrorNotice } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { ApiError, apiFetch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { ROLE_LABELS, ROLES, type Role } from "@/lib/rbac";
import { adminUserListSchema, adminUserSchema, type AdminUser } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

const roleBadge: Record<Role, BadgeVariant> = {
  user: "neutral",
  moderator: "info",
  admin: "accent",
};

export function UserTable() {
  const me = useAuthStore((s) => s.user);
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const users = useQuery({
    queryKey: queryKeys.adminUsers(),
    queryFn: () => apiFetch("/api/v1/admin/users", { schema: adminUserListSchema, auth: true }),
  });

  const changeRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: Role }) =>
      apiFetch(`/api/v1/admin/users/${id}/role`, {
        method: "PATCH",
        body: { role },
        schema: adminUserSchema,
        auth: true,
      }),
    onSuccess: (user) => {
      toast(`${user.username} is now ${ROLE_LABELS[user.role]}`);
      void queryClient.invalidateQueries({ queryKey: queryKeys.adminUsers() });
    },
    onError: (err) => {
      toast(err instanceof ApiError ? err.message : "Could not update role", "error");
    },
  });

  if (users.isPending) {
    return (
      <Card className="flex flex-col gap-3 p-4" data-testid="user-table-loading">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </Card>
    );
  }
  if (users.isError) {
    return <ErrorNotice message={users.error.message} onRetry={() => void users.refetch()} />;
  }

  return (
    <Card className="overflow-hidden p-0" data-testid="user-table">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <h2 className="font-display text-sm font-semibold">
          Users <span className="font-normal text-faint">({users.data.users.length})</span>
        </h2>
        <span className="text-xs text-faint">Change a role to grant or revoke access</span>
      </div>
      <ul className="divide-y divide-border">
        {users.data.users.map((u) => (
          <UserRow
            key={u.id}
            user={u}
            isSelf={u.id === me?.id}
            pending={changeRole.isPending && changeRole.variables?.id === u.id}
            onRoleChange={(role) => changeRole.mutate({ id: u.id, role })}
          />
        ))}
      </ul>
    </Card>
  );
}

// UserRow is presentational (no data fetching) so it is trivially testable.
export function UserRow({
  user,
  isSelf,
  pending,
  onRoleChange,
}: {
  user: AdminUser;
  isSelf: boolean;
  pending: boolean;
  onRoleChange: (role: Role) => void;
}) {
  return (
    <li className="flex items-center gap-3 px-4 py-3" data-testid="user-row">
      <Avatar name={user.username} size="sm" />
      <div className="min-w-0">
        <p className="flex items-center gap-2 truncate text-sm font-medium text-foreground">
          {user.username}
          {isSelf && <span className="text-xs font-normal text-faint">(you)</span>}
        </p>
        <p className="truncate text-xs text-muted">{user.email}</p>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <Badge variant={roleBadge[user.role]} className="hidden sm:inline-flex">
          {ROLE_LABELS[user.role]}
        </Badge>
        <select
          value={user.role}
          disabled={isSelf || pending}
          aria-label={`Role for ${user.username}`}
          data-testid="role-select"
          onChange={(e) => onRoleChange(e.target.value as Role)}
          className="cursor-pointer rounded-[var(--radius)] border border-border-strong bg-surface-2 px-2.5 py-1.5 text-sm text-foreground outline-none focus:border-primary disabled:cursor-not-allowed disabled:opacity-55"
        >
          {ROLES.map((r) => (
            <option key={r} value={r}>
              {ROLE_LABELS[r]}
            </option>
          ))}
        </select>
      </div>
    </li>
  );
}
