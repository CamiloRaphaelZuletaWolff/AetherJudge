"use client";

import { LayoutDashboard, LogOut, ShieldCheck, User as UserIcon } from "lucide-react";
import { useRouter } from "next/navigation";

import { Can } from "@/components/auth/can";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownItem,
  DropdownLabel,
  DropdownLink,
  DropdownMenu,
  DropdownSeparator,
} from "@/components/ui/dropdown";
import { ROLE_LABELS, roleOf } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth";

export function UserMenu() {
  const user = useAuthStore((s) => s.user);
  const signOut = useAuthStore((s) => s.signOut);
  const router = useRouter();
  if (!user) return null;

  return (
    <DropdownMenu
      trigger={
        <span className="flex items-center gap-2 rounded-full p-0.5 transition-opacity hover:opacity-90">
          <Avatar name={user.username} size="sm" />
        </span>
      }
    >
      <div className="flex items-center gap-3 px-2.5 py-2">
        <Avatar name={user.username} size="md" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{user.username}</p>
          <p className="truncate text-xs text-faint">{user.email}</p>
        </div>
      </div>
      <DropdownLabel>{ROLE_LABELS[roleOf(user)]}</DropdownLabel>
      <DropdownSeparator />
      <DropdownLink href="/dashboard">
        <LayoutDashboard className="size-4 text-faint" /> Dashboard
      </DropdownLink>
      <DropdownLink href="/profile">
        <UserIcon className="size-4 text-faint" /> Profile
      </DropdownLink>
      <Can perm="admin.access">
        <DropdownLink href="/admin">
          <ShieldCheck className="size-4 text-faint" /> Admin
        </DropdownLink>
      </Can>
      <DropdownSeparator />
      <DropdownItem
        onClick={() => {
          void signOut().then(() => router.replace("/auth"));
        }}
        className="text-danger hover:bg-danger/10"
      >
        <LogOut className="size-4" /> Sign out
      </DropdownItem>
    </DropdownMenu>
  );
}
