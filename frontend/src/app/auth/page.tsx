"use client";

import { ArrowRight, Check, Code2, Trophy } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { LoginForm, SignupForm } from "@/components/auth/auth-forms";
import { Logo } from "@/components/shell/logo";
import { Card } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Tabs } from "@/components/ui/tabs";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { safeReturnPath } from "@/lib/navigation";
import { useAuthStore } from "@/stores/auth";

type Mode = "signin" | "signup";

function AuthPageInner() {
  const [mode, setMode] = useState<Mode>("signin");
  const status = useAuthStore((s) => s.status);
  const router = useRouter();
  const params = useSearchParams();
  const returnPath = safeReturnPath(params.get("next"));
  useEffect(() => {
    if (status === "authenticated") router.replace(returnPath);
  }, [status, router, returnPath]);
  return (
    <div className="relative flex min-h-screen flex-col">
      <div className="bg-grid pointer-events-none absolute inset-0 opacity-25 [mask-image:linear-gradient(black,transparent)]" />
      <header className="relative mx-auto flex h-20 w-full max-w-7xl items-center px-6 sm:px-8">
        <Logo href="/" />
        <div className="ml-auto flex items-center gap-4">
          <Link href="/" className="text-xs text-muted hover:text-foreground">
            Back to home
          </Link>
          <ThemeToggle />
        </div>
      </header>
      <main className="relative mx-auto grid w-full max-w-5xl flex-1 items-center gap-12 px-6 py-10 lg:grid-cols-2 lg:gap-24">
        <section className="hidden lg:block">
          <span className="mb-6 grid size-14 place-items-center rounded-2xl border border-primary/20 bg-primary/10 text-primary">
            <Trophy className="size-7" />
          </span>
          <p className="eyebrow mb-4">Welcome to the arena</p>
          <h2 className="font-display text-5xl font-semibold leading-tight tracking-tight">
            Big ideas.
            <br />
            <span className="text-primary">One accepted solution.</span>
          </h2>
          <p className="mt-5 max-w-sm text-sm leading-relaxed text-muted">
            A space to challenge yourself, learn from every attempt, and feel the excitement of
            moving up.
          </p>
          <ul className="mt-8 space-y-4 text-sm text-muted">
            {[
              "A ready-to-code workspace",
              "Clear feedback on every submission",
              "Live standings with every solve",
            ].map((item) => (
              <li key={item} className="flex items-center gap-3">
                <Check className="size-4 text-primary" />
                {item}
              </li>
            ))}
          </ul>
          <div className="mt-10 flex items-center gap-2 font-mono text-xs text-faint">
            <Code2 className="size-4" />
            C++ · Python · Go
          </div>
        </section>
        <div className="mx-auto w-full max-w-md">
          <Card className="p-6 sm:p-8">
            <div className="mb-7">
              <p className="eyebrow mb-2">
                {mode === "signin" ? "Pick up where you left off" : "Your first step to the top"}
              </p>
              <h1 className="font-display text-3xl font-semibold tracking-tight">
                {mode === "signin" ? "Welcome back" : "Create your account"}
              </h1>
              <p className="mt-2 text-sm text-muted">
                {mode === "signin"
                  ? "Sign in to enter the arena."
                  : "Choose your handle. Make it known."}
              </p>
            </div>
            <Tabs
              className="mb-6"
              label="Account access"
              value={mode}
              onChange={setMode}
              options={[
                { value: "signin", label: "Sign in", panelId: "auth-form" },
                { value: "signup", label: "Sign up", panelId: "auth-form" },
              ]}
            />
            <div
              id="auth-form"
              role="tabpanel"
              aria-label={mode === "signin" ? "Sign in" : "Sign up"}
            >
              {mode === "signin" ? <LoginForm /> : <SignupForm />}
            </div>
          </Card>
          <p className="mt-5 flex items-center justify-center gap-1.5 text-xs text-muted">
            Your next challenge is waiting <ArrowRight className="size-3" />
          </p>
        </div>
      </main>
    </div>
  );
}
export default function AuthPage() {
  return (
    <Suspense
      fallback={
        <main className="flex min-h-screen items-center justify-center">
          <Spinner />
        </main>
      }
    >
      <AuthPageInner />
    </Suspense>
  );
}
