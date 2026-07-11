"use client";

import { Health, HistoryEntry } from "@/lib/api";

function Row({ ok, label, detail }: { ok: boolean; label: string; detail?: string }) {
  return (
    <div className="flex items-center justify-between gap-2 py-1.5 text-sm">
      <span className="flex items-center gap-2">
        <span
          className={`inline-block h-2 w-2 shrink-0 rounded-full ${
            ok ? "bg-green-500" : "bg-red-500"
          }`}
        />
        {label}
      </span>
      {detail && <span className="truncate text-xs text-[var(--muted)]">{detail}</span>}
    </div>
  );
}

/** Compact status readout for the dashboard sidebar. */
export function StatusPanel({
  health,
  history,
}: {
  health: Health | null;
  history: HistoryEntry[];
}) {
  const sent = history.filter((e) => e.status === "sent").length;
  const failed = history.length - sent;

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
      <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--muted)]">
        Status
      </h2>
      <Row
        ok={!!health?.hasResume}
        label="Resume"
        detail={health?.hasResume ? "attached" : "missing"}
      />
      <Row
        ok={!!health?.hasCredentials}
        label="Gmail"
        detail={health?.gmailUser || (health?.hasCredentials ? "connected" : "not set")}
      />
      <Row
        ok={!!health?.aiEnabled}
        label="AI"
        detail={health?.aiEnabled ? health.aiModel : "template only"}
      />

      <div className="mt-3 grid grid-cols-3 gap-2 border-t border-[var(--border)] pt-3 text-center">
        <Stat value={history.length} label="Total" />
        <Stat value={sent} label="Sent" tone="green" />
        <Stat value={failed} label="Failed" tone={failed ? "red" : undefined} />
      </div>
    </div>
  );
}

function Stat({
  value,
  label,
  tone,
}: {
  value: number;
  label: string;
  tone?: "green" | "red";
}) {
  const color =
    tone === "green"
      ? "text-green-600 dark:text-green-400"
      : tone === "red"
        ? "text-red-500"
        : "text-[var(--foreground)]";
  return (
    <div className="rounded-lg bg-[var(--background)] py-2">
      <div className={`text-lg font-bold ${color}`}>{value}</div>
      <div className="text-[10px] uppercase tracking-wide text-[var(--muted)]">{label}</div>
    </div>
  );
}
