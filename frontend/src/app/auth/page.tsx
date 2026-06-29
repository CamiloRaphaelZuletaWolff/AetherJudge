"use client";

import { motion } from "framer-motion";
import { useRouter } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { LoginForm, SignupForm } from "@/components/auth/auth-forms";
import { Logo } from "@/components/shell/logo";
import { Card } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Tabs } from "@/components/ui/tabs";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { fadeInUp } from "@/lib/motion";
import { useAuthStore } from "@/stores/auth";

type Mode = "signin" | "signup";

function AuthPageInner() {
  const [mode, setMode] = useState<Mode>("signin");
  const status = useAuthStore((s) => s.status);
  const router = useRouter();

  // Already signed in? This page has nothing to offer.
  useEffect(() => {
    if (status === "authenticated") router.replace("/dashboard");
  }, [status, router]);

  return (
    <div className="relative flex min-h-screen flex-col">
      <div className="bg-grid pointer-events-none absolute inset-0 opacity-50 [mask-image:radial-gradient(ellipse_at_center,black,transparent_75%)]" />
      <div className="pointer-events-none absolute -top-40 left-1/2 size-[36rem] -translate-x-1/2 rounded-full bg-primary/10 blur-3xl" />

      <header className="relative mx-auto flex h-16 w-full max-w-6xl items-center px-6">
        <Logo href="/" />
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </header>

      <main className="relative flex flex-1 flex-col items-center justify-center px-6 pb-16">
        <motion.div variants={fadeInUp} initial="hidden" animate="show" className="w-full max-w-sm">
          <div className="mb-6 text-center">
            <h1 className="font-display text-2xl font-semibold tracking-tight">
              {mode === "signin" ? "Welcome back" : "Create your account"}
            </h1>
            <p className="mt-1 text-sm text-muted">
              {mode === "signin"
                ? "Sign in to enter the arena."
                : "Join contests and climb the leaderboard."}
            </p>
          </div>

          <Card className="p-6">
            <div className="mb-5">
              <Tabs
                value={mode}
                onChange={setMode}
                options={[
                  { value: "signin", label: "Sign in" },
                  { value: "signup", label: "Sign up" },
                ]}
              />
            </div>
            {mode === "signin" ? <LoginForm /> : <SignupForm />}
          </Card>

          <p className="mt-5 text-center text-xs text-faint">
            Demo accounts: <span className="font-mono text-muted">alice</span> /{" "}
            <span className="font-mono text-muted">bob</span> · password{" "}
            <span className="font-mono text-muted">password123</span>
          </p>
        </motion.div>
      </main>
    </div>
  );
}

export default function AuthPage() {
  // useSearchParams (inside the forms) requires a Suspense boundary.
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
