"use client";

import { ReactNode, useState } from "react";
import { IconChevronDown, IconX } from "./icons";

/* UI primitives, all styled through the tokens in globals.css. */

// ---- Page ------------------------------------------------------------------

/** Page frame: real app proportions, one entrance on load. */
export function Page({ wide = false, children }: { wide?: boolean; children: ReactNode }) {
  return (
    <main
      className={`enter mx-auto w-full flex-1 px-4 py-6 sm:px-6 lg:px-8 lg:py-8 ${
        wide ? "max-w-[1440px]" : "max-w-[1120px]"
      }`}
    >
      {children}
    </main>
  );
}

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

  const heading = (
    <h2 className="flex min-w-0 items-baseline gap-2 text-[14px] font-medium text-[var(--fg)]">
      {step !== undefined && (
        <span className="font-mono text-[12px] text-[var(--subtle)]">{String(step).padStart(2, "0")}</span>
      )}
      <span className="truncate">{title}</span>
    </h2>
  );

  return (
    <section
      className={`rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--elevated)] shadow-[var(--shadow-sm)] ${className}`}
    >
      {title && (
        <header
          className={`flex min-h-12 items-center justify-between gap-3 px-5 py-2 ${
            showBody ? "border-b border-[var(--border)]" : ""
          }`}
        >
          {collapsible ? (
            <button
              onClick={() => setOpen((o) => !o)}
              aria-expanded={open}
              className="-mx-1 flex min-w-0 flex-1 items-center gap-2 rounded-[var(--radius-sm)] px-1 py-1 text-left"
            >
              <IconChevronDown
                className={`text-[var(--subtle)] transition-transform duration-150 ${open ? "" : "-rotate-90"}`}
              />
              {heading}
            </button>
          ) : (
            heading
          )}
          {action && <div className="flex shrink-0 items-center gap-2">{action}</div>}
        </header>
      )}
      {showBody && <div className="p-5">{children}</div>}
    </section>
  );
}

// ---- Section header --------------------------------------------------------

export function SectionHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-[24px] font-medium leading-tight text-[var(--fg)] sm:text-[28px]">{title}</h1>
        {subtitle && <p className="mt-2 max-w-[42rem] text-[14px] leading-[22px] text-[var(--muted)]">{subtitle}</p>}
      </div>
      {action && <div className="flex flex-wrap items-center gap-2">{action}</div>}
    </div>
  );
}

/** Small heading for a group inside a page, with an optional count. */
export function SubHeading({ children, count }: { children: ReactNode; count?: number }) {
  return (
    <h3 className="mb-3 flex items-baseline gap-2 text-[14px] font-medium text-[var(--fg)]">
      {children}
      {count !== undefined && count > 0 && (
        <span className="font-mono text-[12px] text-[var(--subtle)]">{count}</span>
      )}
    </h3>
  );
}

// ---- Field / inputs --------------------------------------------------------

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-2 block text-[13px] font-medium text-[var(--fg)]">{label}</span>
      {children}
      {hint && <span className="mt-2 block text-[12px] leading-4 text-[var(--subtle)]">{hint}</span>}
    </label>
  );
}

/** Controls fill their container unless the caller passes its own width. */
const fill = (cls?: string) => (/(^|\s)w-/.test(cls ?? "") ? "" : "w-full");

export const controlBase =
  "rounded-[var(--radius-md)] border border-[var(--border-strong)] bg-[var(--surface)] px-3 py-2 text-[14px] text-[var(--fg)] outline-none transition-[border-color,box-shadow] duration-150 placeholder:text-[var(--subtle)] focus:border-[var(--fg)]/40 focus:ring-2 focus:ring-[var(--ring)] disabled:cursor-not-allowed disabled:opacity-60";

export function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${controlBase} ${fill(props.className)} h-9 ${props.className ?? ""}`} />;
}

export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${controlBase} ${fill(props.className)} resize-y leading-relaxed ${props.className ?? ""}`} />;
}

export function Select(props: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      {...props}
      className={`${controlBase} ${fill(props.className)} h-9 cursor-pointer appearance-none bg-no-repeat pr-9 ${props.className ?? ""}`}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%237b7266' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")",
        backgroundPosition: "right 10px center",
        ...props.style,
      }}
    />
  );
}

/** Two or more mutually exclusive options, e.g. the SDE / AI profile. */
export function Segmented<T extends string>({
  value,
  options,
  onChange,
  disabled,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  disabled?: boolean;
}) {
  return (
    <div className="inline-flex h-9 items-center gap-1 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface-sunken)] p-1">
      {options.map((o) => {
        const on = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            disabled={disabled}
            onClick={() => onChange(o.value)}
            aria-pressed={on}
            className={`h-7 rounded-[4px] px-3 text-[13px] font-medium transition-colors duration-150 disabled:opacity-50 ${
              on
                ? "bg-[var(--elevated)] text-[var(--fg)] shadow-[var(--shadow-sm)] ring-1 ring-[var(--border)]"
                : "text-[var(--muted)] hover:text-[var(--fg)]"
            }`}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

// ---- Button ----------------------------------------------------------------

type Variant = "primary" | "secondary" | "ghost" | "danger";

/** Button styling, shared with links that act as buttons. */
export function buttonClass(variant: Variant = "primary", size: "sm" | "md" = "md") {
  const variants = {
    primary: "bg-[var(--primary)] text-[var(--primary-fg)] hover:bg-[var(--primary-hover)]",
    secondary:
      "border border-[var(--border-strong)] bg-[var(--elevated)] text-[var(--fg)] hover:bg-[var(--surface-sunken)]",
    ghost: "text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]",
    danger:
      "border border-[var(--border-strong)] bg-[var(--elevated)] text-[var(--danger-fg)] hover:border-[var(--danger)]/40 hover:bg-[var(--danger-soft)]",
  }[variant];
  const sizes = { sm: "h-8 px-3 text-[13px]", md: "h-9 px-4 text-[14px]" }[size];
  return `inline-flex shrink-0 select-none items-center justify-center gap-2 whitespace-nowrap rounded-[var(--radius-md)] font-medium transition-[background-color,color,border-color,transform] duration-150 active:scale-[0.98] disabled:pointer-events-none disabled:opacity-45 aria-disabled:pointer-events-none aria-disabled:opacity-45 ${variants} ${sizes}`;
}

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  children,
  ...props
}: {
  variant?: Variant;
  size?: "sm" | "md";
  loading?: boolean;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...props}
      disabled={props.disabled || loading}
      className={`${buttonClass(variant, size)} ${props.className ?? ""}`}
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
      className={`inline-block h-3.5 w-3.5 animate-spin rounded-full border-[1.5px] border-current border-t-transparent ${className}`}
      aria-hidden="true"
    />
  );
}

// ---- Badge -----------------------------------------------------------------

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

const toneText: Record<Tone, string> = {
  neutral: "text-[var(--muted)]",
  accent: "text-[var(--accent-soft-fg)]",
  success: "text-[var(--success-fg)]",
  warning: "text-[var(--warning-fg)]",
  danger: "text-[var(--danger-fg)]",
  info: "text-[var(--info-fg)]",
};
const toneDot: Record<Tone, string> = {
  neutral: "bg-[var(--subtle)]",
  accent: "bg-[var(--accent)]",
  success: "bg-[var(--success)]",
  warning: "bg-[var(--warning)]",
  danger: "bg-[var(--danger)]",
  info: "bg-[var(--info)]",
};

/** Quiet tag: hairline box, colored dot only when the tone carries meaning. */
export function Badge({ tone = "neutral", children, className = "" }: { tone?: Tone; children: ReactNode; className?: string }) {
  return (
    <span
      className={`inline-flex h-5 items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] px-1.5 text-[12px] leading-none ${toneText[tone]} ${className}`}
    >
      {tone !== "neutral" && <Dot tone={tone} />}
      {children}
    </span>
  );
}

export function Dot({ tone = "neutral" }: { tone?: Tone }) {
  return <span className={`inline-block h-1.5 w-1.5 shrink-0 rounded-full ${toneDot[tone]}`} aria-hidden="true" />;
}

/** Status dot + label, e.g. connection indicators. */
export function StatusPill({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-[13px] text-[var(--muted)]">
      <Dot tone={ok ? "success" : "danger"} />
      {label}
    </span>
  );
}

// ---- Postmark (the signature detail) ---------------------------------------

export function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

/** Mono timestamp in a thin stamp frame. Sent items get the vermilion ink. */
export function Postmark({ iso, label, sent = false }: { iso?: string; label?: string; sent?: boolean }) {
  return (
    <time
      dateTime={iso}
      className={`inline-flex h-5 shrink-0 items-center whitespace-nowrap rounded-[4px] border px-1.5 font-mono text-[11px] leading-none ${
        sent
          ? "border-[var(--accent)]/45 text-[var(--accent-soft-fg)]"
          : "border-[var(--border-strong)] text-[var(--subtle)]"
      }`}
    >
      {label ?? (iso ? formatWhen(iso) : "")}
    </time>
  );
}

// ---- Notice (inline status message) ----------------------------------------

/** Inline message with a tone dot. Replaces tinted banners. */
export function Notice({ tone = "neutral", children, className = "" }: { tone?: Tone; children: ReactNode; className?: string }) {
  return (
    <div
      className={`flex items-start gap-3 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface-sunken)] px-3 py-2 text-[13px] leading-5 text-[var(--muted)] ${className}`}
    >
      <span className="mt-[7px]">
        <Dot tone={tone} />
      </span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

// ---- Empty state -----------------------------------------------------------

export function EmptyState({ icon, title, children }: { icon?: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-[var(--radius-lg)] border border-dashed border-[var(--border-strong)] px-6 py-12 text-center">
      {icon && <div className="mb-3 text-[var(--subtle)]">{icon}</div>}
      <p className="text-[14px] font-medium text-[var(--fg)]">{title}</p>
      {children && <p className="mt-1 max-w-sm text-[13px] leading-5 text-[var(--muted)]">{children}</p>}
    </div>
  );
}

/** One-line empty row inside a list. */
export function EmptyRow({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-[var(--radius-md)] border border-dashed border-[var(--border-strong)] px-4 py-6 text-center text-[13px] text-[var(--muted)]">
      {children}
    </div>
  );
}

// ---- Skeleton --------------------------------------------------------------

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`rounded-[var(--radius-md)] bg-[var(--surface-sunken)] ${className}`} />;
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
  const tone: Tone = kind === "success" ? "success" : kind === "error" ? "danger" : "info";
  return (
    <div
      role="status"
      className="flex items-start gap-3 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--elevated)] px-4 py-3 text-[14px] text-[var(--fg)] shadow-[var(--shadow-sm)]"
    >
      <span className="mt-[8px]">
        <Dot tone={tone} />
      </span>
      <span className="min-w-0 flex-1 whitespace-pre-wrap leading-[22px]">{message}</span>
      {onClose && (
        <button
          onClick={onClose}
          className="-mr-1 shrink-0 rounded-[var(--radius-sm)] p-1 text-[var(--subtle)] transition-colors hover:text-[var(--fg)]"
          aria-label="Dismiss"
        >
          <IconX />
        </button>
      )}
    </div>
  );
}

/** Stack of page-level alerts with the standard gap below. */
export function Alerts({ children }: { children: ReactNode }) {
  return <div className="mb-6 space-y-3 empty:hidden">{children}</div>;
}
