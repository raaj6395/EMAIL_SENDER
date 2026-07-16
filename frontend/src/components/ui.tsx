"use client";

import { ReactNode, useState } from "react";

/* ============================================================================
   UI primitives — all styled through the design tokens in globals.css.
   Existing prop APIs (Card title/step/action/collapsible, Button variant/
   loading, Field/Input/Textarea/Toast) are preserved; visuals upgraded and
   new primitives (Select, Badge, Spinner, EmptyState, Skeleton, SectionHeader)
   added.
   ========================================================================== */

// ---- Card ------------------------------------------------------------------

export function Card({
  title,
  step,
  action,
  collapsible = false,
  defaultOpen = true,
  children,
  className = "",
}: {
  title?: string;
  step?: number;
  action?: ReactNode;
  collapsible?: boolean;
  defaultOpen?: boolean;
  children: ReactNode;
  className?: string;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const showBody = !collapsible || open;

  return (
    <section
      className={`rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--elevated)] shadow-[var(--shadow-sm)] ${className}`}
    >
      {title && (
        <header
          className="flex items-center justify-between gap-3 px-5 py-4"
          style={{ borderBottom: showBody ? "1px solid var(--border)" : "none" }}
        >
          <h2 className="flex items-center gap-2.5 text-[13px] font-semibold tracking-tight text-[var(--fg)]">
            {step !== undefined && (
              <span className="flex h-[22px] w-[22px] items-center justify-center rounded-[var(--radius-full)] bg-[var(--accent)] text-[11px] font-bold text-[var(--accent-fg)]">
                {step}
              </span>
            )}
            {title}
          </h2>
          <div className="flex items-center gap-2">
            {action}
            {collapsible && (
              <button
                onClick={() => setOpen((o) => !o)}
                className="rounded-[var(--radius-sm)] px-2 py-1 text-xs font-medium text-[var(--muted)] transition-colors hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
                aria-expanded={open}
              >
                {open ? "Hide" : "Show"}
              </button>
            )}
          </div>
        </header>
      )}
      {showBody && <div className="p-5">{children}</div>}
    </section>
  );
}

// ---- Section header (for page/section titles) ------------------------------

export function SectionHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-lg font-semibold tracking-tight text-[var(--fg)] sm:text-xl">{title}</h1>
        {subtitle && <p className="mt-1 max-w-2xl text-sm text-[var(--muted)]">{subtitle}</p>}
      </div>
      {action && <div className="flex items-center gap-2">{action}</div>}
    </div>
  );
}

// ---- Field / inputs --------------------------------------------------------

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
      <span className="mb-1.5 block text-[13px] font-medium text-[var(--fg)]">{label}</span>
      {children}
      {hint && <span className="mt-1.5 block text-xs leading-relaxed text-[var(--muted)]">{hint}</span>}
    </label>
  );
}

const controlBase =
  "w-full rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--fg)] shadow-[var(--shadow-sm)] outline-none transition-colors placeholder:text-[var(--subtle)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--ring)] disabled:cursor-not-allowed disabled:opacity-60";

export function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${controlBase} ${props.className ?? ""}`} />;
}

export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${controlBase} resize-y leading-relaxed ${props.className ?? ""}`} />;
}

export function Select(props: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      {...props}
      className={`${controlBase} cursor-pointer appearance-none bg-no-repeat pr-9 ${props.className ?? ""}`}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%235b6472' stroke-width='2.5' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E\")",
        backgroundPosition: "right 10px center",
        ...props.style,
      }}
    />
  );
}

// ---- Button ----------------------------------------------------------------

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  children,
  ...props
}: {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md";
  loading?: boolean;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const variants = {
    primary:
      "bg-[var(--accent)] text-[var(--accent-fg)] shadow-[var(--shadow-sm)] hover:bg-[var(--accent-hover)] active:bg-[var(--accent-active)] disabled:opacity-50",
    secondary:
      "border border-[var(--border-strong)] bg-[var(--elevated)] text-[var(--fg)] shadow-[var(--shadow-sm)] hover:bg-[var(--elevated-hover)] disabled:opacity-50",
    ghost:
      "text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)] disabled:opacity-50",
    danger:
      "bg-[var(--danger)] text-white shadow-[var(--shadow-sm)] hover:opacity-90 disabled:opacity-50",
  }[variant];
  const sizes = { sm: "px-3 py-1.5 text-[13px]", md: "px-4 py-2 text-sm" }[size];

  return (
    <button
      {...props}
      disabled={props.disabled || loading}
      className={`inline-flex select-none items-center justify-center gap-2 rounded-[var(--radius-md)] font-medium transition-colors disabled:cursor-not-allowed ${variants} ${sizes} ${props.className ?? ""}`}
    >
      {loading && <Spinner />}
      {children}
    </button>
  );
}

// ---- Spinner ---------------------------------------------------------------

export function Spinner({ className = "" }: { className?: string }) {
  return (
    <span
      className={`inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent ${className}`}
      aria-hidden="true"
    />
  );
}

// ---- Badge -----------------------------------------------------------------

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

const toneClasses: Record<Tone, string> = {
  neutral: "bg-[var(--surface-sunken)] text-[var(--muted)]",
  accent: "bg-[var(--accent-soft)] text-[var(--accent-soft-fg)]",
  success: "bg-[var(--success-soft)] text-[var(--success-fg)]",
  warning: "bg-[var(--warning-soft)] text-[var(--warning-fg)]",
  danger: "bg-[var(--danger-soft)] text-[var(--danger-fg)]",
  info: "bg-[var(--info-soft)] text-[var(--info-fg)]",
};

export function Badge({
  tone = "neutral",
  children,
  className = "",
}: {
  tone?: Tone;
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-[var(--radius-full)] px-2 py-0.5 text-[11px] font-medium ${toneClasses[tone]} ${className}`}
    >
      {children}
    </span>
  );
}

/** A status dot + label chip, e.g. connection/health indicators. */
export function StatusPill({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-[var(--radius-full)] px-2.5 py-1 text-xs font-medium ${
        ok ? "bg-[var(--success-soft)] text-[var(--success-fg)]" : "bg-[var(--danger-soft)] text-[var(--danger-fg)]"
      }`}
    >
      <span className={`inline-block h-1.5 w-1.5 rounded-full ${ok ? "bg-[var(--success)]" : "bg-[var(--danger)]"}`} />
      {label}
    </span>
  );
}

// ---- Empty state -----------------------------------------------------------

export function EmptyState({
  icon,
  title,
  children,
}: {
  icon?: ReactNode;
  title: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-[var(--radius-lg)] border border-dashed border-[var(--border-strong)] px-6 py-10 text-center">
      {icon && <div className="mb-2 text-2xl opacity-70">{icon}</div>}
      <p className="text-sm font-medium text-[var(--fg)]">{title}</p>
      {children && <p className="mt-1 max-w-sm text-xs leading-relaxed text-[var(--muted)]">{children}</p>}
    </div>
  );
}

// ---- Skeleton --------------------------------------------------------------

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-[var(--radius-md)] bg-[var(--surface-sunken)] ${className}`} />;
}

// ---- Toast -----------------------------------------------------------------

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
    success: "border-[var(--success)]/30 bg-[var(--success-soft)] text-[var(--success-fg)]",
    error: "border-[var(--danger)]/30 bg-[var(--danger-soft)] text-[var(--danger-fg)]",
    info: "border-[var(--accent)]/30 bg-[var(--accent-soft)] text-[var(--accent-soft-fg)]",
  }[kind];
  return (
    <div
      role="status"
      className={`flex items-start justify-between gap-3 rounded-[var(--radius-md)] border px-4 py-3 text-sm shadow-[var(--shadow-sm)] ${styles}`}
    >
      <span className="whitespace-pre-wrap leading-relaxed">{message}</span>
      {onClose && (
        <button onClick={onClose} className="shrink-0 opacity-60 transition-opacity hover:opacity-100" aria-label="Dismiss">
          ✕
        </button>
      )}
    </div>
  );
}
