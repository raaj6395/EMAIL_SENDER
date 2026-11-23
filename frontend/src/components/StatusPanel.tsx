"use client";

import { HistoryEntry } from "@/lib/api";

/** Send totals: three figures, sent in postmark ink. */
export function StatusPanel({ history }: { history: HistoryEntry[] }) {
  const sent = history.filter((e) => e.status === "sent").length;
  const failed = history.length - sent;

  return (
    <div className="grid grid-cols-3 divide-x divide-[var(--border)] rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--elevated)] shadow-[var(--shadow-sm)]">
      <Stat value={sent} label="Sent" accent />
      <Stat value={failed} label="Failed" />
      <Stat value={history.length} label="All time" />
    </div>
  );
}

function Stat({ value, label, accent }: { value: number; label: string; accent?: boolean }) {
  return (
    <div className="px-4 py-4">
      <div className="text-[13px] text-[var(--muted)]">{label}</div>
      <div className={`mt-1 text-[28px] font-medium leading-none tabular-nums ${accent ? "text-[var(--accent)]" : "text-[var(--fg)]"}`}>
        {value.toLocaleString()}
      </div>
    </div>
  );
}
