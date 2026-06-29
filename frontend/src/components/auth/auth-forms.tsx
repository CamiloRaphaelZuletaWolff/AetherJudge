"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { AlertCircle } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/input";
import { ApiError, apiFetch } from "@/lib/api";
import { authResponseSchema, type AuthResponse } from "@/lib/schemas";
import { useAuthStore } from "@/stores/auth";

// Validation mirrors the backend's rules (internal/api/validate.go) so users
// see errors before the round-trip; the server remains authoritative.
const signupSchema = z.object({
  username: z
    .string()
    .regex(/^[A-Za-z0-9_]{3,32}$/, "3-32 characters: letters, digits, underscore"),
  email: z.string().email("enter a valid email address"),
  password: z.string().min(8, "at least 8 characters").max(72, "at most 72 characters"),
});
type SignupValues = z.infer<typeof signupSchema>;

const loginSchema = z.object({
  login: z.string().min(1, "enter your username or email"),
  password: z.string().min(1, "enter your password"),
});
type LoginValues = z.infer<typeof loginSchema>;

function useOnSignedIn() {
  const signedIn = useAuthStore((s) => s.signedIn);
  const router = useRouter();
  const params = useSearchParams();

  return (resp: AuthResponse) => {
    signedIn(resp);
    const next = params.get("next");
    router.replace(next && next.startsWith("/") ? next : "/dashboard");
  };
}

function FormError({ message, testid }: { message: string; testid?: string }) {
  return (
    <p
      data-testid={testid}
      className="flex items-center gap-2 rounded-[var(--radius)] border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-danger"
    >
      <AlertCircle className="size-4 shrink-0" />
      {message}
    </p>
  );
}

export function SignupForm() {
  const onSignedIn = useOnSignedIn();
  const form = useForm<SignupValues>({ resolver: zodResolver(signupSchema) });

  const mutation = useMutation({
    mutationFn: (values: SignupValues) =>
      apiFetch("/api/v1/auth/signup", { method: "POST", body: values, schema: authResponseSchema }),
    onSuccess: onSignedIn,
    onError: (error) => {
      if (error instanceof ApiError && error.code === "username_taken") {
        form.setError("username", { message: error.message });
      } else if (error instanceof ApiError && error.code === "email_taken") {
        form.setError("email", { message: error.message });
      } else {
        form.setError("root", { message: error.message });
      }
    },
  });

  return (
    <form
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      className="flex flex-col gap-4"
      noValidate
    >
      <Field
        label="Username"
        autoComplete="username"
        error={form.formState.errors.username?.message}
        {...form.register("username")}
      />
      <Field
        label="Email"
        type="email"
        autoComplete="email"
        error={form.formState.errors.email?.message}
        {...form.register("email")}
      />
      <Field
        label="Password"
        type="password"
        autoComplete="new-password"
        error={form.formState.errors.password?.message}
        {...form.register("password")}
      />
      {form.formState.errors.root && <FormError message={form.formState.errors.root.message!} />}
      <Button type="submit" size="lg" loading={mutation.isPending} data-testid="signup-submit">
        Create account
      </Button>
    </form>
  );
}

export function LoginForm() {
  const onSignedIn = useOnSignedIn();
  const form = useForm<LoginValues>({ resolver: zodResolver(loginSchema) });

  const mutation = useMutation({
    mutationFn: (values: LoginValues) =>
      apiFetch("/api/v1/auth/login", { method: "POST", body: values, schema: authResponseSchema }),
    onSuccess: onSignedIn,
    onError: (error) => {
      form.setError("root", {
        message: error instanceof ApiError ? error.message : "could not sign in",
      });
    },
  });

  return (
    <form
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      className="flex flex-col gap-4"
      noValidate
    >
      <Field
        label="Username or email"
        autoComplete="username"
        error={form.formState.errors.login?.message}
        {...form.register("login")}
      />
      <Field
        label="Password"
        type="password"
        autoComplete="current-password"
        error={form.formState.errors.password?.message}
        {...form.register("password")}
      />
      {form.formState.errors.root && (
        <FormError message={form.formState.errors.root.message!} testid="login-error" />
      )}
      <Button type="submit" size="lg" loading={mutation.isPending} data-testid="login-submit">
        Sign in
      </Button>
    </form>
  );
}
