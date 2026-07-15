"use client";

import { useEffect, useState } from "react";
import { ApiError, HRContact, api } from "@/lib/api";
import { Button, Card, Textarea, Toast } from "@/components/ui";
import { HRPager, HRToolbar, RankBadge, SentSection, useHRContacts } from "@/components/hr";

// Default outreach message. {name} and {company} are filled per contact.
const DEFAULT_TEMPLATE = `Hi {name},

I am Ankit Raj, a 2026 graduate from NIT Allahabad. I am reaching out about Software Engineer opportunities at {company}.

I interned as a Backend Engineer at Carousell and Propel, where I built and shipped production backend features across 10+ Golang microservices, optimized database queries, and contributed to service migrations including moving a Django monolith to Go gRPC microservices. My core stack is Go and Node.js, with gRPC, Protobuf, REST, PostgreSQL, MySQL, MongoDB, Redis, and Kafka, deployed with Docker and monitored with Prometheus and Grafana. I am also familiar with Kubernetes.

A few highlights: cut query time by 90% on 11M+ records via composite indexing in Postgres, implemented secure auth with JWT and refresh-token rotation, and maintain a 1800+ LeetCode rating.

I would appreciate the chance to talk if there is a fit. I am sharing my resume for reference.

Thank you,
Ankit Raj

Email: ankitraj210922@gmail.com
Contact: 6386830484`;

function fillTemplate(tpl: string, c: HRContact): string {
  const name = c.name?.trim() || "there";
  return tpl.replaceAll("{name}", name).replaceAll("{company}", c.company);
}

/** One WhatsApp contact row: details + editable message + Send (opens wa.me). */
function WhatsAppRow({
  contact,
  template,
  onSent,
  blocked,
}: {
  contact: HRContact;
  template: string;
  onSent: (c: HRContact) => void;
  blocked: boolean; // true when the send-rate limit is active
}) {
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
            href={!blocked && waLink ? waLink : undefined}
            target="_blank"
            rel="noopener noreferrer"
            aria-disabled={blocked || !waLink}
            onClick={(e) => {
              if (blocked || !waLink) {
                e.preventDefault(); // rate-limited or no phone → don't open/record
                return;
              }
              onSent(contact);
            }}
            className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium text-white transition ${
              !blocked && waLink
                ? "bg-green-600 hover:opacity-90"
                : "pointer-events-none bg-green-600/40"
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
  const { contacts, sent, rate, setRate, total, page, totalPages, q, setQ, loading, error, goto, reload } =
    useHRContacts(api.hrWhatsApp);

  useEffect(() => {
    api
      .health()
      .then((h) => setHrEnabled(h.hrEnabled))
      .catch(() => setHrEnabled(false));
  }, []);

  // Tick the cooldown down locally every second so the countdown is live and the
  // Send buttons re-enable on their own when it hits zero.
  useEffect(() => {
    if (!rate || rate.cooldownLeft <= 0) return;
    const t = setInterval(() => {
      setRate((r) =>
        r && r.cooldownLeft > 0
          ? { ...r, cooldownLeft: r.cooldownLeft - 1, blocked: r.capReached || r.cooldownLeft - 1 > 0 }
          : r
      );
    }, 1000);
    return () => clearInterval(t);
  }, [rate, setRate]);

  const capReached = rate?.capReached ?? false;
  const cooldownLeft = rate?.cooldownLeft ?? 0;
  const blocked = capReached || cooldownLeft > 0;

  // Clicking "Send on WhatsApp" records the contact as sent (server enforces the
  // rate limit), then refreshes so it moves into the Sent section.
  const handleSent = async (c: HRContact) => {
    try {
      const res = await api.hrMarkSent({
        channel: "whatsapp",
        company: c.company,
        name: c.name,
        role: c.role,
        phone: c.phone,
        key: c.waPhone,
      });
      if (res.rate) setRate(res.rate); // starts the cooldown
      setToast(`Moved ${c.name || c.company} to Sent.`);
      reload();
    } catch (e) {
      // The server blocks a too-fast/over-cap send with a 429 + message.
      setToast(e instanceof ApiError ? e.message : "Could not record the send.");
      reload(); // refresh rate status
    }
  };

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
            {/* Send-rate guard: keeps you under WhatsApp's spam radar. */}
            {rate && (
              <div
                className={`mb-3 rounded-lg border px-3 py-2 text-sm ${
                  capReached
                    ? "border-red-500/40 bg-red-500/10 text-red-700 dark:text-red-300"
                    : cooldownLeft > 0
                    ? "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300"
                    : "border-[var(--border)] bg-[var(--background)] text-[var(--muted)]"
                }`}
              >
                {capReached ? (
                  <>
                    <strong>Daily limit reached</strong> ({rate.sentToday}/{rate.dailyCap}). Pause
                    until tomorrow so your number doesn’t get flagged.
                  </>
                ) : cooldownLeft > 0 ? (
                  <>
                    Cooling down — next message in <strong>{cooldownLeft}s</strong>. Sent today:{" "}
                    {rate.sentToday}/{rate.dailyCap}.
                  </>
                ) : (
                  <>
                    Safe to send. Today: <strong>{rate.sentToday}/{rate.dailyCap}</strong> · a short
                    pause is enforced between messages to avoid getting flagged.
                  </>
                )}
              </div>
            )}

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
                <WhatsAppRow
                  key={`${c.waPhone}-${i}`}
                  contact={c}
                  template={template}
                  onSent={handleSent}
                  blocked={blocked}
                />
              ))}
              {!loading && contacts.length === 0 && !error && (
                <div className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center text-sm text-[var(--muted)]">
                  No matching contacts.
                </div>
              )}
            </div>

            <HRPager page={page} totalPages={totalPages} loading={loading} goto={goto} />

            <SentSection sent={sent} />
          </div>
        </Card>
      )}
    </main>
  );
}
