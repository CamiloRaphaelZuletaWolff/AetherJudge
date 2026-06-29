"use client";

import { useParams } from "next/navigation";

import { ContestBuilder } from "@/components/admin/contest-builder";
import { AuthGate } from "@/components/auth/auth-gate";
import { AppShell } from "@/components/shell/app-shell";

export default function ContestBuilderPage() {
  const params = useParams<{ id: string }>();
  return (
    <AuthGate requirePermission="admin.access">
      <AppShell>
        <ContestBuilder contestId={params.id} />
      </AppShell>
    </AuthGate>
  );
}
