// Minimal hand-rolled UI kit: six primitives, one dark theme, zero
// component-library dependencies.
"use client";

import {
  forwardRef,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
} from "react";

function cx(...parts: Array<string | false | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

const buttonVariants: Record<ButtonVariant, string> = {
  primary: "bg-emerald-600 hover:bg-emerald-500 text-white disabled:bg-emerald-900",
  secondary: "bg-zinc-800 hover:bg-zinc-700 text-zinc-100 border border-zinc-700",
  ghost: "hover:bg-zinc-800 text-zinc-300",
  danger: "bg-red-700 hover:bg-red-600 text-white",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "primary", loading = false, className, children, disabled, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={cx(
        "inline-flex items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium",
        "transition-colors disabled:cursor-not-allowed disabled:opacity-60",
        buttonVariants[variant],
        className,
      )}
      {...props}
    >
      {loading && <Spinner className="size-4" />}
      {children}
    </button>
  );
});

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
}

export const Field = forwardRef<HTMLInputElement, FieldProps>(function Field(
  { label, error, id, className, ...props },
  ref,
) {
  const inputId = id ?? label.toLowerCase().replace(/\s+/g, "-");
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={inputId} className="text-sm font-medium text-zinc-300">
        {label}
      </label>
      <input
        ref={ref}
        id={inputId}
        className={cx(
          "rounded-md border bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none",
          "placeholder:text-zinc-600 focus:border-emerald-600",
          error ? "border-red-700" : "border-zinc-700",
          className,
        )}
        aria-invalid={error ? true : undefined}
        {...props}
      />
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
});

export function Card({
  className,
  children,
  ...props
}: { className?: string; children: ReactNode } & React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cx("rounded-lg border border-zinc-800 bg-zinc-950/60 p-5", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export function Badge({
  tone,
  className,
  children,
  ...props
}: {
  tone: "green" | "red" | "amber" | "purple" | "orange" | "sky" | "zinc";
  className?: string;
  children: ReactNode;
} & React.HTMLAttributes<HTMLSpanElement>) {
  const tones = {
    green: "bg-emerald-950 text-emerald-300 border-emerald-800",
    red: "bg-red-950 text-red-300 border-red-800",
    amber: "bg-amber-950 text-amber-300 border-amber-800",
    purple: "bg-purple-950 text-purple-300 border-purple-800",
    orange: "bg-orange-950 text-orange-300 border-orange-800",
    sky: "bg-sky-950 text-sky-300 border-sky-800",
    zinc: "bg-zinc-900 text-zinc-300 border-zinc-700",
  };
  return (
    <span
      className={cx(
        "inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium",
        tones[tone],
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export function Tabs<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  onChange: (value: T) => void;
}) {
  return (
    <div role="tablist" className="flex gap-1 rounded-lg border border-zinc-800 bg-zinc-900 p-1">
      {options.map((opt) => (
        <button
          key={opt.value}
          role="tab"
          aria-selected={opt.value === value}
          onClick={() => onChange(opt.value)}
          className={cx(
            "flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
            opt.value === value ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-zinc-200",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

export function Spinner({ className }: { className?: string }) {
  return (
    <svg
      className={cx("animate-spin text-current", className ?? "size-5")}
      viewBox="0 0 24 24"
      fill="none"
      aria-label="loading"
    >
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  );
}

export function ErrorNotice({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Card className="flex flex-col items-center gap-3 text-center">
      <p className="text-sm text-red-300">{message}</p>
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          Try again
        </Button>
      )}
    </Card>
  );
}
