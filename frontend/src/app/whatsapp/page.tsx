"use client";

import { useEffect, useState } from "react";
import { HRContact, api } from "@/lib/api";
import { Button, Card, Textarea, Toast } from "@/components/ui";
import { HRPager, HRToolbar, RankBadge, useHRContacts } from "@/components/hr";

// Default outreach message. {name} and {company} are filled per contact.
const DEFAULT_TEMPLATE =
  "Hi {name}, I hope you're doing well. I'm a 2026 B.Tech graduate exploring software engineering opportunities at {company}. I'd love to share my resume and learn about any suitable openings. Thank you!";

function fillTemplate(tpl: string, c: HRContact): string {
  const name = c.name?.trim() || "there";
  return tpl.replaceAll("{name}", name).replaceAll("{company}", c.company);
}

/** One WhatsApp contact row: details + editable message + Send (opens wa.me). */
function WhatsAppRow({ contact, template }: { contact: HRContact; template: string }) {
  const [open, setOpen] = useState(false);
  const [msg, setMsg] = useState(() => fillTemplate(template, contact));

  // Keep the message in sync with the shared template until the user edits it.
  const [edited, setEdited] = useState(false);
  useEffect(() => {
    if (!edited) setMsg(fillTemplate(template, contact));
  }, [template, contact, edited]);

  const waLink = contact.waPhone
    ? `https://wa.me/${contact.waPhone}?text=${encodeURIComponent(msg)}`
    : "";

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{contact.name || "(no name)"}</span>
            <RankBadge rank={contact.rank} />
          </div>
          <div className="mt-0.5 truncate text-xs text-[var(--muted)]">
            {[contact.company, contact.role].filter(Boolean).join(" · ")} · {contact.phone}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="secondary" onClick={() => setOpen((o) => !o)}>
            {open ? "Hide" : "Edit"}
          </Button>
          <a
            href={waLink || undefined}
            target="_blank"
            rel="noopener noreferrer"
            aria-disabled={!waLink}
            className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium text-white transition ${
              waLink ? "bg-green-600 hover:opacity-90" : "pointer-events-none bg-green-600/40"
            }`}
          >
            Send on WhatsApp ↗
          </a>
        </div>
      </div>

      {open && (
        <div className="mt-3">
          <Textarea
            value={msg}
            onChange={(e) => {
              setEdited(true);
              setMsg(e.target.value);
            }}
            rows={4}
          />
          <p className="mt-1 text-xs text-[var(--muted)]">
            This opens WhatsApp with the message pre-filled — you still press send there.
          </p>
        </div>
      )}
    </div>
  );
}

export default function WhatsAppPage() {
  const [hrEnabled, setHrEnabled] = useState<boolean | null>(null);
  const [template, setTemplate] = useState(DEFAULT_TEMPLATE);
  const [toast, setToast] = useState<string | null>(null);
  const { contacts, total, page, totalPages, q, setQ, loading, error, goto } = useHRContacts(
    api.hrWhatsApp
  );

  useEffect(() => {
    api
      .health()
      .then((h) => setHrEnabled(h.hrEnabled))
      .catch(() => setHrEnabled(false));
  }, []);

  return (
    <main className="mx-auto w-full max-w-4xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
      <header className="mb-6 border-b border-[var(--border)] pb-5">
        <h1 className="text-xl font-bold tracking-tight sm:text-2xl">WhatsApp outreach</h1>
        <p className="mt-1 text-sm text-[var(--muted)]">
          HR contacts with phone numbers, most important companies first. Click{" "}
          <span className="font-medium">Send on WhatsApp</span> to open WhatsApp with your message
          ready — including the person’s name and company.
        </p>
      </header>

      {toast && (
        <div className="mb-5">
          <Toast kind="info" message={toast} onClose={() => setToast(null)} />
        </div>
      )}

      {hrEnabled === false ? (
        <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700 dark:text-amber-300">
          No HR data found. Place your spreadsheet at{" "}
          <span className="font-mono">backend/data/HR DATA (1).xlsx</span> and restart the backend.
        </div>
      ) : (
        <Card title="Message template">
          <Textarea
            value={template}
            onChange={(e) => setTemplate(e.target.value)}
            rows={3}
            className="mb-1"
          />
          <p className="text-xs text-[var(--muted)]">
            Use <span className="font-mono">{"{name}"}</span> and{" "}
            <span className="font-mono">{"{company}"}</span> — they’re filled per contact. Edit any
            single message before sending with its <span className="font-medium">Edit</span> button.
          </p>

          <div className="mt-4">
            <HRToolbar
              q={q}
              setQ={setQ}
              total={total}
              loading={loading}
              placeholder="Search company, name, role…"
            />

            {error && (
              <div className="mb-3 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-700 dark:text-red-300">
                {error}
              </div>
            )}

            <div className="space-y-3">
              {contacts.map((c, i) => (
                <WhatsAppRow key={`${c.waPhone}-${i}`} contact={c} template={template} />
              ))}
              {!loading && contacts.length === 0 && !error && (
                <div className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center text-sm text-[var(--muted)]">
                  No matching contacts.
                </div>
              )}
            </div>

            <HRPager page={page} totalPages={totalPages} loading={loading} goto={goto} />
          </div>
        </Card>
      )}
    </main>
  );
}
