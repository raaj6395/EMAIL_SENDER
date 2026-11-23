"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import {
  IconBriefcase,
  IconBuilding,
  IconChat,
  IconInbox,
  IconMail,
  IconMailCheck,
  IconMenu,
  IconMoon,
  IconSend,
  IconSun,
  IconUsers,
  IconX,
} from "./icons";

type NavLink = { href: string; label: string; icon: React.ReactNode };

const GROUPS: { label: string; links: NavLink[] }[] = [
  {
    label: "Send",
    links: [
      { href: "/", label: "Email sender", icon: <IconMail /> },
      { href: "/check", label: "Check emails", icon: <IconMailCheck /> },
      { href: "/replies", label: "Replies", icon: <IconInbox /> },
    ],
  },
  {
    label: "Find",
    links: [
      { href: "/jobs", label: "Job search", icon: <IconBriefcase /> },
      { href: "/targets", label: "Target companies", icon: <IconBuilding /> },
      { href: "/email", label: "Email HR", icon: <IconUsers /> },
      { href: "/reach", label: "Reach", icon: <IconSend /> },
      { href: "/whatsapp", label: "WhatsApp", icon: <IconChat /> },
    ],
  },
];

function isActive(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(href + "/");
}

/** Left sidebar on desktop, top bar with a drop-down menu on mobile. */
export function NavBar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => setMobileOpen(false), [pathname]);

  const nav = (
    <div className="flex flex-col gap-6">
      {GROUPS.map((g) => (
        <div key={g.label}>
          <div className="mb-2 px-2.5 text-[12px] text-[var(--subtle)]">{g.label}</div>
          <div className="flex flex-col gap-0.5">
            {g.links.map((l) => {
              const active = isActive(pathname, l.href);
              return (
                <Link
                  key={l.href}
                  href={l.href}
                  aria-current={active ? "page" : undefined}
                  className={`flex h-8 items-center gap-2.5 rounded-[var(--radius-md)] border px-2.5 text-[14px] transition-colors duration-150 ${
                    active
                      ? "border-[var(--border)] bg-[var(--elevated)] font-medium text-[var(--fg)] shadow-[var(--shadow-sm)]"
                      : "border-transparent text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
                  }`}
                >
                  <span className={active ? "text-[var(--accent)]" : "text-[var(--subtle)]"}>{l.icon}</span>
                  {l.label}
                </Link>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );

  return (
    <>
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 flex-col border-r border-[var(--border)] bg-[var(--surface)] lg:flex">
        <div className="flex h-16 items-center px-5">
          <Brand />
        </div>
        <nav className="flex-1 overflow-y-auto px-3 py-2">{nav}</nav>
        <div className="border-t border-[var(--border)] p-3">
          <ThemeToggle />
        </div>
      </aside>

      <div className="sticky top-0 z-30 border-b border-[var(--border)] bg-[var(--surface)] lg:hidden">
        <div className="flex h-14 items-center justify-between px-4">
          <Brand />
          <div className="flex items-center gap-1">
            <ThemeToggle compact />
            <button
              onClick={() => setMobileOpen((o) => !o)}
              className="rounded-[var(--radius-md)] p-2 text-[var(--muted)] transition-colors hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
              aria-label="Menu"
              aria-expanded={mobileOpen}
            >
              {mobileOpen ? <IconX size={18} /> : <IconMenu size={18} />}
            </button>
          </div>
        </div>
        {mobileOpen && <nav className="border-t border-[var(--border)] px-3 py-4">{nav}</nav>}
      </div>
    </>
  );
}

/** Wordmark plus a flat envelope mark in postmark ink. */
function Brand() {
  return (
    <Link href="/" className="flex items-center gap-2">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <rect x="2.5" y="5" width="19" height="14" rx="2" stroke="var(--accent)" strokeWidth="1.75" />
        <path d="m3.5 6.5 8.5 6 8.5-6" stroke="var(--accent)" strokeWidth="1.75" strokeLinejoin="round" />
      </svg>
      <span className="text-[16px] font-medium text-[var(--fg)]">Job Hunt</span>
    </Link>
  );
}

function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const [theme, setTheme] = useState<"light" | "dark" | null>(null);

  useEffect(() => {
    const stored = localStorage.getItem("theme") as "light" | "dark" | null;
    const initial = stored ?? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    setTheme(initial);
    document.documentElement.setAttribute("data-theme", initial);
  }, []);

  const toggle = () => {
    const next = theme === "dark" ? "light" : "dark";
    setTheme(next);
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem("theme", next);
    } catch {
      /* private mode: the toggle still works for this visit */
    }
  };

  if (theme === null) return compact ? <span className="h-9 w-9" /> : <span className="block h-8" />;

  const label = theme === "dark" ? "Light theme" : "Dark theme";
  const icon = theme === "dark" ? <IconSun /> : <IconMoon />;
  if (compact) {
    return (
      <button
        onClick={toggle}
        className="rounded-[var(--radius-md)] p-2 text-[var(--muted)] transition-colors hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
        aria-label={label}
      >
        {icon}
      </button>
    );
  }
  return (
    <button
      onClick={toggle}
      className="flex h-8 w-full items-center gap-2.5 rounded-[var(--radius-md)] px-2.5 text-[14px] text-[var(--muted)] transition-colors hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
    >
      <span className="text-[var(--subtle)]">{icon}</span>
      {label}
    </button>
  );
}
