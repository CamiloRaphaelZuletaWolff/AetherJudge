"use client";

import { ChevronLeft } from "lucide-react";
import Link from "next/link";

import { ContestForm } from "@/components/admin/contest-form";
import { AuthGate } from "@/components/auth/auth-gate";
import { AppShell } from "@/components/shell/app-shell";

export default function NewContestPage() {
  return (
    <AuthGate requirePermission="admin.access">
      <AppShell>
        <Link
          href="/admin"
          className="mb-4 inline-flex items-center gap-1 text-sm text-muted transition-colors hover:text-foreground"
        >
          <ChevronLeft className="size-4" /> Back to admin
        </Link>
        <h1 className="mb-1 font-display text-2xl font-semibold tracking-tight">New contest</h1>
        <p className="mb-6 text-sm text-muted">
          Schedule it now — you&apos;ll add problems and test cases on the next screen.
        </p>
        <div className="max-w-2xl">
          <ContestForm />
        </div>
      </AppShell>
    </AuthGate>
  );
}
