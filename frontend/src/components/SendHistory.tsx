"use client";

import { HistoryEntry } from "@/lib/api";
import { Button, Card } from "./ui";

function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function SendHistory({
  entries,
  digestEnabled,
  digestTo,
  onDigest,
  digesting,
}: {
  entries: HistoryEntry[];
  digestEnabled: boolean;
  digestTo: string;
  onDigest: () => void;
  digesting: boolean;
}) {
  const sent = entries.filter((e) => e.status === "sent").length;
  const failed = entries.length - sent;

  return (
    <Card title="Track — send history">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-[var(--muted)]">
          {entries.length === 0 ? (
            "No emails sent yet."
          ) : (
            <>
              <span className="font-medium text-[var(--fg)]">{entries.length}</span> total ·{" "}
              <span className="text-[var(--success-fg)]">{sent} sent</span>
              {failed > 0 && (
                <>
                  {" "}
                  · <span className="text-[var(--danger-fg)]">{failed} failed</span>
                </>
              )}
            </>
          )}
        </div>
        {digestEnabled && (
          <Button variant="secondary" onClick={onDigest} loading={digesting} disabled={entries.length === 0}>
            Email digest{digestTo ? ` → ${digestTo}` : ""}
          </Button>
        )}
      </div>

      {entries.length > 0 && (
        <ul className="divide-y divide-[var(--border)]">
          {entries.map((e, i) => (
            <li key={i} className="flex items-start justify-between gap-4 py-3 text-sm">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-block h-2 w-2 shrink-0 rounded-[var(--radius-full)] ${
                      e.status === "sent" ? "bg-[var(--success)]" : "bg-[var(--danger)]"
                    }`}
                  />
                  <span className="font-medium">{e.company}</span>
                  <span className="truncate text-[var(--muted)]">→ {e.recipientEmail}</span>
                </div>
                <div className="mt-0.5 truncate pl-4 text-xs text-[var(--muted)]">{e.subject}</div>
                {e.status === "failed" && e.error && (
                  <div className="mt-0.5 pl-4 text-xs text-[var(--danger-fg)]">{e.error}</div>
                )}
              </div>
              <span className="shrink-0 whitespace-nowrap text-xs text-[var(--muted)]">
                {formatWhen(e.sentAt)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
