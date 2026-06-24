"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { LoginForm, SignupForm } from "@/components/auth/auth-forms";
import { Card, Spinner, Tabs } from "@/components/ui/ui";
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
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 px-6">
      <Link href="/" className="text-3xl font-bold tracking-tight">
        Arena
      </Link>
      <Card className="w-full max-w-sm">
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
      <p className="text-xs text-zinc-500">
        Demo accounts: <span className="font-mono">alice</span> /{" "}
        <span className="font-mono">bob</span> with password{" "}
        <span className="font-mono">password123</span> (after <code>task db:seed</code>)
      </p>
    </main>
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
