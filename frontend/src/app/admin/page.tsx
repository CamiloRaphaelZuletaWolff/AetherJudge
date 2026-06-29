"use client";

import { ShieldCheck } from "lucide-react";
import { useState } from "react";

import { ContestsAdmin } from "@/components/admin/contests-admin";
import { UserTable } from "@/components/admin/user-table";
import { AuthGate } from "@/components/auth/auth-gate";
import { AppShell } from "@/components/shell/app-shell";
import { Tabs } from "@/components/ui/tabs";

type Tab = "users" | "contests";

const tabs = [
  { value: "users", label: "Users" },
  { value: "contests", label: "Contests" },
] as const;

export default function AdminPage() {
  const [tab, setTab] = useState<Tab>("users");

  // Gated by permission: signed-in users without admin.access get the friendly
  // "Access restricted" screen from AuthGate. The API enforces this for real —
  // every /api/v1/admin route checks the matching permission server-side.
  return (
    <AuthGate requirePermission="admin.access">
      <AppShell>
        <div className="mb-6 flex items-center gap-3">
          <span className="grid size-10 place-items-center rounded-[var(--radius)] bg-accent/10 text-accent">
            <ShieldCheck className="size-5" />
          </span>
          <div>
            <h1 className="font-display text-2xl font-semibold tracking-tight">Admin</h1>
            <p className="text-sm text-muted">
              Manage members, contests, and problems. Administrators only.
            </p>
          </div>
        </div>

        <Tabs value={tab} onChange={setTab} options={tabs} className="mb-6 max-w-xs" />

        {tab === "users" ? <UserTable /> : <ContestsAdmin />}
      </AppShell>
    </AuthGate>
  );
}
