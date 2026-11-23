"use client";

import { useEffect, useState } from "react";
import { ApiError, HRContact, api } from "@/lib/api";
import { Alerts, Button, Card, EmptyRow, Notice, Page, SectionHeader, Textarea, Toast, buttonClass } from "@/components/ui";
import { ContactRow, HRPager, HRToolbar, RankBadge, RowList, SentSection, useHRContacts } from "@/components/hr";
import { IconExternal } from "@/components/icons";

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
    <div>
      <ContactRow
        actions={
          <>
          <Button variant="secondary" size="sm" onClick={() => setOpen((o) => !o)}>
            {open ? "Done" : "Edit message"}
          </Button>
          <a
            href={!blocked && waLink ? waLink : undefined}
            target="_blank"
            rel="noopener noreferrer"
            aria-disabled={blocked || !waLink}
            onClick={(e) => {
              if (blocked || !waLink) {
                e.preventDefault(); // rate-limited or no phone: don't open or record
                return;
              }
              onSent(contact);
            }}
            className={buttonClass("primary", "sm")}
          >
            Open in WhatsApp
            <IconExternal size={14} />
          </a>
          </>
        }
      >
        <div className="flex flex-wrap items-baseline gap-x-3">
          <span className="text-[14px] font-medium">{contact.name || "No name"}</span>
          <RankBadge rank={contact.rank} />
        </div>
        <div className="mt-0.5 truncate text-[13px] text-[var(--muted)]">
          {[contact.company, contact.role].filter(Boolean).join(" · ")}
        </div>
        <div className="mt-0.5 font-mono text-[12px] text-[var(--subtle)]">{contact.phone}</div>
      </ContactRow>

      {open && (
        <div className="px-4 pb-4">
          <Textarea
            value={msg}
            onChange={(e) => {
              setEdited(true);
              setMsg(e.target.value);
            }}
            rows={4}
          />
          <p className="mt-2 text-[12px] text-[var(--subtle)]">Opens WhatsApp with this message filled in. You still press send there</p>
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

  // Sends are blocked only when the 6-hour window cap is reached — there is no
  // per-message cooldown.
  const capReached = rate?.capReached ?? false;
  const blocked = capReached;

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
      setToast(`Moved ${c.name || c.company} to Sent`);
      reload();
    } catch (e) {
      // The server blocks a too-fast/over-cap send with a 429 + message.
      setToast(e instanceof ApiError ? e.message : "Could not record the send.");
      reload(); // refresh rate status
    }
  };

  return (
    <Page>
      <SectionHeader
        title="WhatsApp"
        subtitle="HR contacts with a phone number, most important companies first. Each opens a WhatsApp chat with your message filled in, name and company included"
      />

      <Alerts>{toast && <Toast kind="info" message={toast} onClose={() => setToast(null)} />}</Alerts>

      {hrEnabled === false ? (
        <Notice tone="warning">
          No HR data found. Put your spreadsheet at{" "}
          <span className="font-mono">backend/data/HR DATA (1).xlsx</span> and restart the backend
        </Notice>
      ) : (
        <Card title="Message template">
          <Textarea
            value={template}
            onChange={(e) => setTemplate(e.target.value)}
            rows={6}
          />
          <p className="mt-2 text-[12px] leading-4 text-[var(--subtle)]">
            <span className="font-mono">{"{name}"}</span> and <span className="font-mono">{"{company}"}</span> are filled
            per contact. Edit message changes one contact only
          </p>

          <div className="mt-6">
            {/* Send-rate guard: a rolling 6-hour cap keeps you under WhatsApp's
                spam radar. No per-message cooldown — send back-to-back until the
                cap is reached. */}
            {rate && (
              <Notice tone={capReached ? "danger" : "neutral"} className="mb-4">
                {capReached ? (
                  <>
                    <span className="text-[var(--fg)]">
                      Limit reached, {rate.windowCap} in {rate.windowHours}h
                    </span>
                    . Wait about {Math.ceil(rate.resetIn / 60)} min so your number is not flagged
                  </>
                ) : (
                  <>
                    <span className="font-mono text-[var(--fg)]">
                      {rate.sentInWindow} / {rate.windowCap}
                    </span>{" "}
                    sent in the last {rate.windowHours}h. Send freely until the cap
                  </>
                )}
              </Notice>
            )}

            <HRToolbar
              q={q}
              setQ={setQ}
              total={total}
              loading={loading}
              placeholder="Search company, name or role"
            />

            {error && (
              <Notice tone="danger" className="mb-4">
                {error}
              </Notice>
            )}

            <RowList>
              {contacts.map((c, i) => (
                <WhatsAppRow
                  key={`${c.waPhone}-${i}`}
                  contact={c}
                  template={template}
                  onSent={handleSent}
                  blocked={blocked}
                />
              ))}
            </RowList>
            {!loading && contacts.length === 0 && !error && <EmptyRow>No matching contacts</EmptyRow>}

            <HRPager page={page} totalPages={totalPages} loading={loading} goto={goto} />

            <SentSection sent={sent} />
          </div>
        </Card>
      )}
    </Page>
  );
}
