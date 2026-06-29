"use client";

import { LayoutDashboard, ShieldCheck, User as UserIcon, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { Can } from "@/components/auth/can";
import { Logo } from "@/components/shell/logo";
import { UserMenu } from "@/components/shell/user-menu";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { cn } from "@/lib/cn";
import type { Permission } from "@/lib/rbac";

interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  perm?: Permission;
}

const NAV: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/profile", label: "Profile", icon: UserIcon },
  { href: "/admin", label: "Admin", icon: ShieldCheck, perm: "admin.access" },
];

function NavLink({ item }: { item: NavItem }) {
  const pathname = usePathname();
  const active = pathname === item.href || pathname.startsWith(item.href + "/");
  const Icon = item.icon;
  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      className={cn(
        "inline-flex items-center gap-2 rounded-[var(--radius)] px-3 py-1.5 text-sm font-medium transition-colors",
        active ? "bg-foreground/5 text-foreground" : "text-muted hover:text-foreground",
      )}
    >
      <Icon className="size-4" />
      <span className="hidden sm:inline">{item.label}</span>
    </Link>
  );
}

export function TopNav() {
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-14 w-full max-w-6xl items-center gap-2 px-4 sm:px-6">
        <Logo />
        <nav className="ml-4 flex items-center gap-1">
          {NAV.map((item) =>
            item.perm ? (
              <Can key={item.href} perm={item.perm}>
                <NavLink item={item} />
              </Can>
            ) : (
              <NavLink key={item.href} item={item} />
            ),
          )}
        </nav>
        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          <UserMenu />
        </div>
      </div>
    </header>
  );
}

// AppShell wraps authenticated, chrome-bearing pages (dashboard, profile,
// admin). The contest room uses its own compact header instead.
export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen">
      <TopNav />
      <main className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">{children}</main>
    </div>
  );
}
