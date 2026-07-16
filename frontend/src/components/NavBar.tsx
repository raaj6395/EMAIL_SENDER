"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

type NavLink = { href: string; label: string; icon: React.ReactNode };

const LINKS: NavLink[] = [
  { href: "/", label: "Email Sender", icon: <IconMail /> },
  { href: "/replies", label: "Replies", icon: <IconInbox /> },
  { href: "/jobs", label: "Job Search", icon: <IconBriefcase /> },
  { href: "/email", label: "Email HR", icon: <IconContacts /> },
  { href: "/whatsapp", label: "WhatsApp", icon: <IconChat /> },
];

function isActive(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(href + "/");
}

/** Left sidebar on desktop, top bar with a slide-down menu on mobile. */
export function NavBar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  // Close the mobile menu whenever the route changes.
  useEffect(() => setMobileOpen(false), [pathname]);

  const navItems = LINKS.map((l) => {
    const active = isActive(pathname, l.href);
    return (
      <Link
        key={l.href}
        href={l.href}
        className={`flex items-center gap-3 rounded-[var(--radius-md)] px-3 py-2 text-sm font-medium transition-colors ${
          active
            ? "bg-[var(--accent-soft)] text-[var(--accent-soft-fg)]"
            : "text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
        }`}
      >
        <span className={active ? "text-[var(--accent)]" : "text-[var(--subtle)]"}>{l.icon}</span>
        {l.label}
      </Link>
    );
  });

  return (
    <>
      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 flex-col border-r border-[var(--border)] bg-[var(--elevated)] lg:flex">
        <div className="flex h-16 items-center gap-2.5 px-5">
          <Brand />
        </div>
        <nav className="flex flex-1 flex-col gap-1 px-3 py-2">{navItems}</nav>
        <div className="border-t border-[var(--border)] p-3">
          <ThemeToggle />
        </div>
      </aside>

      {/* Mobile top bar */}
      <div className="sticky top-0 z-30 border-b border-[var(--border)] bg-[var(--elevated)] lg:hidden">
        <div className="flex h-14 items-center justify-between px-4">
          <Brand />
          <div className="flex items-center gap-1">
            <ThemeToggle compact />
            <button
              onClick={() => setMobileOpen((o) => !o)}
              className="rounded-[var(--radius-md)] p-2 text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
              aria-label="Toggle menu"
              aria-expanded={mobileOpen}
            >
              {mobileOpen ? <IconClose /> : <IconMenu />}
            </button>
          </div>
        </div>
        {mobileOpen && (
          <nav className="flex flex-col gap-1 border-t border-[var(--border)] px-3 py-3">{navItems}</nav>
        )}
      </div>
    </>
  );
}

function Brand() {
  return (
    <span className="flex items-center gap-2.5">
      <span className="flex h-7 w-7 items-center justify-center rounded-[var(--radius-md)] bg-[var(--accent)] text-[var(--accent-fg)]">
        <IconBolt />
      </span>
      <span className="text-[15px] font-semibold tracking-tight text-[var(--fg)]">Job Hunt</span>
    </span>
  );
}

// ---- theme toggle ----------------------------------------------------------

function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const [theme, setTheme] = useState<"light" | "dark" | null>(null);

  useEffect(() => {
    const stored = localStorage.getItem("theme") as "light" | "dark" | null;
    const initial =
      stored ?? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    setTheme(initial);
    document.documentElement.setAttribute("data-theme", initial);
  }, []);

  const toggle = () => {
    const next = theme === "dark" ? "light" : "dark";
    setTheme(next);
    document.documentElement.setAttribute("data-theme", next);
    localStorage.setItem("theme", next);
  };

  if (theme === null) {
    // Avoid a hydration flash — render a neutral placeholder until mounted.
    return compact ? <span className="h-9 w-9" /> : <span className="h-9" />;
  }

  const label = theme === "dark" ? "Light mode" : "Dark mode";
  if (compact) {
    return (
      <button
        onClick={toggle}
        className="rounded-[var(--radius-md)] p-2 text-[var(--muted)] hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
        aria-label={label}
      >
        {theme === "dark" ? <IconSun /> : <IconMoon />}
      </button>
    );
  }
  return (
    <button
      onClick={toggle}
      className="flex w-full items-center gap-3 rounded-[var(--radius-md)] px-3 py-2 text-sm font-medium text-[var(--muted)] transition-colors hover:bg-[var(--surface-sunken)] hover:text-[var(--fg)]"
    >
      <span className="text-[var(--subtle)]">{theme === "dark" ? <IconSun /> : <IconMoon />}</span>
      {label}
    </button>
  );
}

// ---- icons (inline, currentColor, 18px) ------------------------------------

function svg(children: React.ReactNode) {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      {children}
    </svg>
  );
}
function IconMail() { return svg(<><rect x="2" y="4" width="20" height="16" rx="2" /><path d="m22 7-10 5L2 7" /></>); }
function IconBriefcase() { return svg(<><rect x="2" y="7" width="20" height="14" rx="2" /><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2" /></>); }
function IconContacts() { return svg(<><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>); }
function IconChat() { return svg(<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />); }
function IconInbox() { return svg(<><path d="M22 12h-6l-2 3h-4l-2-3H2" /><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" /></>); }
function IconBolt() { return svg(<path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z" />); }
function IconMenu() { return svg(<><line x1="4" y1="6" x2="20" y2="6" /><line x1="4" y1="12" x2="20" y2="12" /><line x1="4" y1="18" x2="20" y2="18" /></>); }
function IconClose() { return svg(<><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></>); }
function IconSun() { return svg(<><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></>); }
function IconMoon() { return svg(<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />); }
