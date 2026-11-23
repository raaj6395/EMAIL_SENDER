"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, HRContact, HRPage, HRRateStatus, HRSentRecord } from "@/lib/api";
import { Button, Input, Postmark, SubHeading } from "./ui";
import { IconChevronLeft, IconChevronRight, IconSearch } from "./icons";

/**
 * useHRContacts loads one page of HR contacts (WhatsApp or Email) with debounced
 * search + pagination. `fetcher` is api.hrWhatsApp or api.hrEmail.
 */
export function useHRContacts(
  fetcher: (opts?: { page?: number; pageSize?: number; q?: string }) => Promise<HRPage>
) {
  const pageSize = 50;
  const [contacts, setContacts] = useState<HRContact[]>([]);
  const [sent, setSent] = useState<HRSentRecord[]>([]);
  const [rate, setRate] = useState<HRRateStatus | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [q, setQ] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (p: number, query: string) => {
      setLoading(true);
      setError(null);
      try {
        const res = await fetcher({ page: p, pageSize, q: query });
        setContacts(res.contacts ?? []);
        setSent(res.sent ?? []);
        if (res.rate) setRate(res.rate);
        setTotal(res.total ?? 0);
        setPage(res.page ?? p);
      } catch (e) {
        setError(
          e instanceof ApiError ? e.message : "Could not load contacts. Is the backend running?"
        );
        setContacts([]);
        setTotal(0);
      } finally {
        setLoading(false);
      }
    },
    [fetcher]
  );

  // Initial load + debounced reload on search change (resets to page 1).
  useEffect(() => {
    const t = setTimeout(() => load(1, q), q ? 350 : 0);
    return () => clearTimeout(t);
  }, [q, load]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const goto = (p: number) => load(Math.min(Math.max(1, p), totalPages), q);

  return { contacts, sent, rate, setRate, total, page, totalPages, q, setQ, loading, error, goto, reload: () => load(page, q) };
}

/** Search box + result count, shared by both HR pages. */
export function HRToolbar({
  q,
  setQ,
  total,
  loading,
  placeholder,
  right,
}: {
  q: string;
  setQ: (v: string) => void;
  total: number;
  loading: boolean;
  placeholder: string;
  right?: React.ReactNode;
}) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-3">
      <SearchInput value={q} onChange={setQ} placeholder={placeholder} />
      <span className="font-mono text-[12px] text-[var(--subtle)]">
        {loading ? "loading" : `${total.toLocaleString()} contact${total === 1 ? "" : "s"}`}
      </span>
      {right}
    </div>
  );
}

/** Search field with a leading icon. */
export function SearchInput({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <div className="relative min-w-52 flex-1">
      <IconSearch className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--subtle)]" />
      <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} className="pl-9" />
    </div>
  );
}

/** One row in a contact list: details on the left, actions on the right. */
export function ContactRow({ children, actions }: { children: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3 px-4 py-4">
      <div className="min-w-0 flex-1">{children}</div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}

/** Hairline-divided list container for ContactRow. */
export function RowList({ children }: { children: React.ReactNode }) {
  return (
    <div className="divide-y divide-[var(--border)] overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] empty:hidden">
      {children}
    </div>
  );
}

/** Previous / page / Next controls. */
export function HRPager({
  page,
  totalPages,
  loading,
  goto,
}: {
  page: number;
  totalPages: number;
  loading: boolean;
  goto: (p: number) => void;
}) {
  if (totalPages <= 1) return null;
  return (
    <div className="mt-4 flex items-center justify-between gap-3">
      <Button variant="secondary" size="sm" onClick={() => goto(page - 1)} disabled={loading || page <= 1}>
        <IconChevronLeft />
        Previous
      </Button>
      <span className="font-mono text-[12px] text-[var(--subtle)]">
        {page} / {totalPages}
      </span>
      <Button variant="secondary" size="sm" onClick={() => goto(page + 1)} disabled={loading || page >= totalPages}>
        Next
        <IconChevronRight />
      </Button>
    </div>
  );
}

/** Company importance (AI-ranked, 0 to 100). Top companies read in primary text. */
export function RankBadge({ rank }: { rank: number }) {
  return (
    <span
      className={`font-mono text-[12px] ${rank >= 85 ? "text-[var(--fg)]" : "text-[var(--subtle)]"}`}
      title="Company importance, AI-ranked"
    >
      rank {rank}
    </span>
  );
}

/** Contacts already reached on this channel, each with its postmark. */
export function SentSection({ sent }: { sent: HRSentRecord[] }) {
  return (
    <div className="mt-8">
      <SubHeading count={sent.length}>Sent</SubHeading>
      {sent.length === 0 ? (
        <p className="text-[13px] text-[var(--muted)]">Nothing sent yet. Contacts move here once you reach out</p>
      ) : (
        <RowList>
          {sent.map((s, i) => (
            <div key={`${s.key}-${i}`} className="flex items-center justify-between gap-4 px-4 py-3">
              <div className="min-w-0">
                <div className="flex items-baseline gap-2">
                  <span className="truncate text-[14px] font-medium">{s.name || "No name"}</span>
                  <span className="truncate text-[13px] text-[var(--muted)]">{[s.company, s.role].filter(Boolean).join(" · ")}</span>
                </div>
                <div className="mt-0.5 truncate font-mono text-[12px] text-[var(--subtle)]">{s.email || s.phone}</div>
              </div>
              <Postmark iso={s.sentAt} sent />
            </div>
          ))}
        </RowList>
      )}
    </div>
  );
}
