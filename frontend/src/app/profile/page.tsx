"use client";

import { ArrowRight, CalendarDays, Code2, FileCode2, Link2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { AuthGate } from "@/components/auth/auth-gate";
import { StatCard } from "@/components/profile/stat-card";
import { SettingsForm } from "@/components/profile/settings-form";
import { AppShell } from "@/components/shell/app-shell";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { buttonStyles } from "@/components/ui/button";
import { Tabs } from "@/components/ui/tabs";
import { ROLE_LABELS, roleOf, type Role } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth";
import { useProfileStore } from "@/stores/profile";
import { useEditorStore } from "@/stores/editor";

const roleVariant: Record<Role, "neutral" | "info" | "accent"> = {
  user: "neutral",
  moderator: "info",
  admin: "accent",
};

type Tab = "overview" | "submissions" | "settings";

export default function ProfilePage() {
  return (
    <AuthGate>
      <AppShell>
        <ProfileInner />
      </AppShell>
    </AuthGate>
  );
}

function ProfileInner() {
  const user = useAuthStore((s) => s.user);
  const { displayName, bio, website } = useProfileStore();
  const drafts = useEditorStore((s) => s.drafts);
  const [tab, setTab] = useState<Tab>("overview");
  if (!user) return null;

  const role = roleOf(user);
  const joined = new Date(user.created_at).toLocaleDateString(undefined, {
    year: "numeric",
    month: "long",
  });

  return (
    <div className="flex flex-col gap-6">
      {/* Identity header */}
      <Card className="relative flex flex-col items-start gap-5 overflow-hidden border-t-4 border-t-primary p-6 sm:flex-row sm:items-center sm:p-8">
        <Avatar name={user.username} size="xl" />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="font-display text-2xl font-semibold tracking-tight">
              {displayName || user.username}
            </h1>
            <Badge variant={roleVariant[role]}>{ROLE_LABELS[role]}</Badge>
          </div>
          <p className="mt-0.5 text-sm text-muted">@{user.username}</p>
          {bio && <p className="mt-3 max-w-prose text-sm text-muted">{bio}</p>}
          <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-faint">
            <span className="inline-flex items-center gap-1.5">
              <CalendarDays className="size-3.5" /> Joined {joined}
            </span>
            {/^https?:\/\//i.test(website) && (
              <a
                href={website}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 text-accent hover:underline"
              >
                <Link2 className="size-3.5" /> {website.replace(/^https?:\/\//, "")}
              </a>
            )}
          </div>
        </div>
      </Card>

      <div className="max-w-md">
        <Tabs
          value={tab}
          onChange={setTab}
          options={[
            { value: "overview", label: "Overview" },
            { value: "submissions", label: "Submissions" },
            { value: "settings", label: "Settings" },
          ]}
        />
      </div>

      {tab === "overview" && (
        <div className="grid gap-4 sm:grid-cols-3">
          <StatCard
            icon={FileCode2}
            label="Saved drafts on this device"
            value={String(Object.keys(drafts).length)}
          />
          <StatCard
            icon={Code2}
            label="Languages in your drafts"
            value={String(new Set(Object.keys(drafts).map((key) => key.split(":").at(-1))).size)}
          />
          <StatCard icon={CalendarDays} label="Member since" value={joined} />
          <Card className="flex flex-wrap items-center justify-between gap-4 p-6 sm:col-span-3">
            <div>
              <h2 className="font-display font-semibold">Keep your momentum.</h2>
              <p className="mt-1 text-sm text-muted">
                Your code drafts are saved on this device. Reopen a contest to continue solving.
              </p>
            </div>
            <Link href="/dashboard" className={buttonStyles({ variant: "secondary" })}>
              Explore contests <ArrowRight className="size-4" />
            </Link>
          </Card>
        </div>
      )}

      {tab === "submissions" && (
        <Card className="p-10 text-center">
          <FileCode2 className="mx-auto mb-4 size-8 text-primary" />
          <h2 className="font-display text-lg font-semibold">
            Every attempt is part of the story.
          </h2>
          <p className="mx-auto mb-5 mt-2 max-w-sm text-sm text-muted">
            Open a contest to find your submissions, verdicts, and execution times in My
            submissions.
          </p>
          <Link href="/dashboard" className={buttonStyles({ variant: "secondary" })}>
            Find a contest <ArrowRight className="size-4" />
          </Link>
        </Card>
      )}

      {tab === "settings" && (
        <div className="max-w-xl">
          <SettingsForm />
        </div>
      )}
    </div>
  );
}
