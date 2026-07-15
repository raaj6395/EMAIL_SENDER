"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, HRContact, HRPage } from "@/lib/api";
import { Button, Input } from "./ui";

/**
 * useHRContacts loads one page of HR contacts (WhatsApp or Email) with debounced
 * search + pagination. `fetcher` is api.hrWhatsApp or api.hrEmail.
 */
export function useHRContacts(
  fetcher: (opts?: { page?: number; pageSize?: number; q?: string }) => Promise<HRPage>
) {
  const pageSize = 50;
  const [contacts, setContacts] = useState<HRContact[]>([]);
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

  return { contacts, total, page, totalPages, q, setQ, loading, error, goto, reload: () => load(page, q) };
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
      <div className="min-w-[200px] flex-1">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={placeholder} />
      </div>
      <span className="text-sm text-[var(--muted)]">
        {loading ? "Loading…" : `${total.toLocaleString()} contact${total === 1 ? "" : "s"}`}
      </span>
      {right}
    </div>
  );
}

/** Prev / page / Next controls. */
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
    <div className="mt-4 flex items-center justify-center gap-3 text-sm">
      <Button variant="secondary" onClick={() => goto(page - 1)} disabled={loading || page <= 1}>
        ← Prev
      </Button>
      <span className="text-[var(--muted)]">
        Page {page} of {totalPages}
      </span>
      <Button variant="secondary" onClick={() => goto(page + 1)} disabled={loading || page >= totalPages}>
        Next →
      </Button>
    </div>
  );
}

/** Colored badge for a company's importance rank. */
export function RankBadge({ rank }: { rank: number }) {
  const tone =
    rank >= 85
      ? "bg-green-500/15 text-green-700 dark:text-green-300"
      : rank >= 60
      ? "bg-blue-500/15 text-blue-700 dark:text-blue-300"
      : rank >= 40
      ? "bg-[var(--background)] text-[var(--muted)]"
      : "bg-[var(--background)] text-[var(--muted)]";
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${tone}`} title="Company importance (AI-ranked)">
      ★ {rank}
    </span>
  );
}
