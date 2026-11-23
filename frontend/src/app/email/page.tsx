"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { HRContact, api, setComposePrefill } from "@/lib/api";
import { Button, Card, Notice, Page, SectionHeader } from "@/components/ui";
import { ContactRow, HRPager, HRToolbar, RankBadge, RowList, SentSection, useHRContacts } from "@/components/hr";
import { EmptyRow } from "@/components/ui";

/** One email-contact row: details + a button that prefills the compose form. */
function EmailRow({ contact, onCompose }: { contact: HRContact; onCompose: (c: HRContact) => void }) {
  return (
    <ContactRow
      actions={
        <Button size="sm" onClick={() => onCompose(contact)}>
          Compose
        </Button>
      }
    >
      <div className="flex flex-wrap items-baseline gap-x-3">
        <span className="text-[14px] font-medium">{contact.name || "No name"}</span>
        <RankBadge rank={contact.rank} />
      </div>
      <div className="mt-0.5 truncate text-[13px] text-[var(--muted)]">
        {[contact.company, contact.role].filter(Boolean).join(" · ")}
      </div>
      <div className="mt-0.5 truncate font-mono text-[12px] text-[var(--subtle)]">{contact.email}</div>
    </ContactRow>
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
    <Page>
      <SectionHeader
        title="Email HR"
        subtitle="HR contacts with an email address, most important companies first. Compose opens the sender with the recipient filled in, then preview and send as usual. Ranks fill in as you browse, or Re-rank all scores everything at once"
      />

      {hrEnabled === false ? (
        <Notice tone="warning">
          No HR data found. Put your spreadsheet at{" "}
          <span className="font-mono">backend/data/HR DATA (1).xlsx</span> and restart the backend
        </Notice>
      ) : (
        <Card title="Email contacts">
          <HRToolbar
            q={q}
            setQ={setQ}
            total={total}
            loading={loading}
            placeholder="Search company, name, role or email"
            right={
              <Button variant="secondary" size="sm" onClick={handleRerank} loading={reranking}>
                {reranking ? "Ranking" : "Re-rank all"}
              </Button>
            }
          />

          {error && (
            <Notice tone="danger" className="mb-4">
              {error}
            </Notice>
          )}

          <RowList>
            {contacts.map((c, i) => (
              <EmailRow key={`${c.email}-${i}`} contact={c} onCompose={handleCompose} />
            ))}
          </RowList>
          {!loading && contacts.length === 0 && !error && <EmptyRow>No matching contacts</EmptyRow>}

          <HRPager page={page} totalPages={totalPages} loading={loading} goto={goto} />

          <SentSection sent={sent} />
        </Card>
      )}
    </Page>
  );
}
