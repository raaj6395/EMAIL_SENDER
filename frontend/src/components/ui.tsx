"use client";

import { ReactNode } from "react";

export function Card({
  title,
  step,
  children,
  className = "",
}: {
  title?: string;
  step?: number;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      className={`rounded-xl border border-[var(--border)] bg-[var(--card)] p-5 shadow-sm ${className}`}
    >
      {title && (
        <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-[var(--muted)]">
          {step !== undefined && (
            <span className="flex h-5 w-5 items-center justify-center rounded-full bg-[var(--accent)] text-[11px] font-bold text-white">
              {step}
            </span>
          )}
          {title}
        </h2>
      )}
      {children}
    </section>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-[var(--muted)]">{hint}</span>}
    </label>
  );
}

const inputBase =
  "w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm outline-none transition focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20";

export function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${inputBase} ${props.className ?? ""}`} />;
}

export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${inputBase} resize-y ${props.className ?? ""}`} />;
}

export function Button({
  variant = "primary",
  loading = false,
  children,
  ...props
}: {
  variant?: "primary" | "secondary" | "ghost";
  loading?: boolean;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const styles = {
    primary:
      "bg-[var(--accent)] text-white hover:opacity-90 disabled:opacity-50",
    secondary:
      "border border-[var(--border)] bg-[var(--card)] hover:bg-[var(--background)] disabled:opacity-50",
    ghost: "text-[var(--muted)] hover:text-[var(--foreground)] disabled:opacity-50",
  }[variant];
  return (
    <button
      {...props}
      disabled={props.disabled || loading}
      className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition ${styles} ${props.className ?? ""}`}
    >
      {loading && (
        <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
      )}
      {children}
    </button>
  );
}

export function Toast({
  kind,
  message,
  onClose,
}: {
  kind: "success" | "error" | "info";
  message: string;
  onClose?: () => void;
}) {
  const styles = {
    success: "border-green-500/40 bg-green-500/10 text-green-700 dark:text-green-300",
    error: "border-red-500/40 bg-red-500/10 text-red-700 dark:text-red-300",
    info: "border-[var(--accent)]/40 bg-[var(--accent)]/10 text-[var(--accent)]",
  }[kind];
  return (
    <div className={`flex items-start justify-between gap-3 rounded-lg border px-4 py-3 text-sm ${styles}`}>
      <span className="whitespace-pre-wrap">{message}</span>
      {onClose && (
        <button onClick={onClose} className="shrink-0 opacity-60 hover:opacity-100" aria-label="Dismiss">
          ✕
        </button>
      )}
    </div>
  );
}
