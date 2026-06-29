"use client";

import { CalendarDays, CheckCircle2, Link2, Trophy } from "lucide-react";
import { useState } from "react";

import { AuthGate } from "@/components/auth/auth-gate";
import { StatCard } from "@/components/profile/stat-card";
import { SettingsForm } from "@/components/profile/settings-form";
import { AppShell } from "@/components/shell/app-shell";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Tabs } from "@/components/ui/tabs";
import { ROLE_LABELS, roleOf, type Role } from "@/lib/rbac";
import { useAuthStore } from "@/stores/auth";
import { useProfileStore } from "@/stores/profile";

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
      <Card className="flex flex-col items-start gap-5 p-6 sm:flex-row sm:items-center">
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
            {website && (
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
          <StatCard icon={CheckCircle2} label="Problems solved" value="—" hint="Problems solved" />
          <StatCard icon={Trophy} label="Best rank" value="—" hint="Best contest rank" />
          <StatCard icon={CalendarDays} label="Contests" value="—" hint="Contests entered" />
          <Card className="p-4 text-sm text-faint sm:col-span-3">
            Stats are placeholders for now — they&apos;ll populate once the profile/stats API is
            wired up. The UI is ready for it.
          </Card>
        </div>
      )}

      {tab === "submissions" && (
        <Card className="p-10 text-center text-sm text-faint">
          A unified submission history across contests will appear here once the backend exposes it.
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
