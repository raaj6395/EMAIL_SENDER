"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { HRContact, api, setComposePrefill } from "@/lib/api";
import { Button, Card } from "@/components/ui";
import { HRPager, HRToolbar, RankBadge, SentSection, useHRContacts } from "@/components/hr";

/** One email-contact row: details + a button that prefills the compose form. */
function EmailRow({ contact, onCompose }: { contact: HRContact; onCompose: (c: HRContact) => void }) {
  return (
    <div className="flex items-start justify-between gap-3 rounded-xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium">{contact.name || "(no name)"}</span>
          <RankBadge rank={contact.rank} />
        </div>
        <div className="mt-0.5 truncate text-xs text-[var(--muted)]">
          {[contact.company, contact.role].filter(Boolean).join(" · ")}
        </div>
        <div className="mt-0.5 truncate font-mono text-xs text-[var(--muted)]">{contact.email}</div>
      </div>
      <button
        onClick={() => onCompose(contact)}
        className="shrink-0 rounded-lg bg-[var(--accent)] px-3 py-2 text-sm font-medium text-white transition hover:opacity-90"
      >
        Compose email →
      </button>
    </div>
  );
}

export default function EmailHRPage() {
  const router = useRouter();
  const [hrEnabled, setHrEnabled] = useState<boolean | null>(null);
  const [reranking, setReranking] = useState(false);
  const { contacts, sent, total, page, totalPages, q, setQ, loading, error, goto, reload } =
    useHRContacts(api.hrEmail);

  const handleRerank = async () => {
    setReranking(true);
    try {
      await api.hrRerank();
      reload();
    } catch {
      /* best-effort; ranking also fills in progressively on its own */
    } finally {
      setReranking(false);
    }
  };

  useEffect(() => {
    api
      .health()
      .then((h) => setHrEnabled(h.hrEnabled))
      .catch(() => setHrEnabled(false));
  }, []);

  // Stash the contact (with an HR-sent marker) and jump to the main compose page.
  // The marker lets the sender move this contact to Sent after the email is sent.
  const handleCompose = (c: HRContact) => {
    setComposePrefill({
      input: {
        recipientEmail: c.email ?? "",
        recipientName: c.name ?? "",
        company: c.company ?? "",
      },
      hrSent: {
        channel: "email",
        company: c.company,
        name: c.name,
        role: c.role,
        email: c.email,
        key: c.email,
      },
    });
    router.push("/");
  };

  return (
    <main className="mx-auto w-full max-w-4xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
      <header className="mb-6 border-b border-[var(--border)] pb-5">
        <h1 className="text-xl font-bold tracking-tight sm:text-2xl">Email outreach</h1>
        <p className="mt-1 text-sm text-[var(--muted)]">
          HR contacts with email addresses, most important companies first. Click{" "}
          <span className="font-medium">Compose email</span> to open the sender with this recipient
          pre-filled — then Preview &amp; Send as usual (AI-tailored, resume attached).
        </p>
        <p className="mt-1 text-xs text-[var(--muted)]">
          Company ranking fills in automatically as you browse. Use{" "}
          <span className="font-medium">Re-rank all</span> to score every company at once (may take a
          minute; it keeps working in the background even if it looks slow).
        </p>
      </header>

      {hrEnabled === false ? (
        <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-sm text-amber-700 dark:text-amber-300">
          No HR data found. Place your spreadsheet at{" "}
          <span className="font-mono">backend/data/HR DATA (1).xlsx</span> and restart the backend.
        </div>
      ) : (
        <Card title="Email contacts">
          <HRToolbar
            q={q}
            setQ={setQ}
            total={total}
            loading={loading}
            placeholder="Search company, name, role, email…"
            right={
              <Button variant="secondary" onClick={handleRerank} loading={reranking}>
                {reranking ? "Ranking…" : "Re-rank all"}
              </Button>
            }
          />

          {error && (
            <div className="mb-3 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-700 dark:text-red-300">
              {error}
            </div>
          )}

          <div className="space-y-3">
            {contacts.map((c, i) => (
              <EmailRow key={`${c.email}-${i}`} contact={c} onCompose={handleCompose} />
            ))}
            {!loading && contacts.length === 0 && !error && (
              <div className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center text-sm text-[var(--muted)]">
                No matching contacts.
              </div>
            )}
          </div>

          <HRPager page={page} totalPages={totalPages} loading={loading} goto={goto} />

          <SentSection sent={sent} />
        </Card>
      )}
    </main>
  );
}
