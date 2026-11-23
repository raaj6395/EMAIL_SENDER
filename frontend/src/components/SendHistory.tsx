"use client";

import { HistoryEntry } from "@/lib/api";
import { Button, Card, Postmark } from "./ui";

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
  return (
    <Card
      title="Sent mail"
      action={
        digestEnabled && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onDigest}
            loading={digesting}
            disabled={entries.length === 0}
            title={digestTo ? `Email a digest to ${digestTo}` : undefined}
          >
            Email digest
          </Button>
        )
      }
    >
      {entries.length === 0 ? (
        <p className="text-[13px] text-[var(--muted)]">Nothing sent yet. Every email you send is logged here</p>
      ) : (
        <ul className="-my-3 divide-y divide-[var(--border)]">
          {entries.map((e, i) => {
            const failed = e.status !== "sent";
            return (
              <li key={i} className="py-3">
                <div className="flex items-center justify-between gap-3">
                  <span className="truncate text-[14px] font-medium text-[var(--fg)]">{e.company}</span>
                  {failed ? (
                    <span className="shrink-0 text-[12px] text-[var(--danger-fg)]">Failed</span>
                  ) : (
                    <Postmark iso={e.sentAt} sent />
                  )}
                </div>
                <div className="mt-0.5 truncate font-mono text-[12px] text-[var(--subtle)]">{e.recipientEmail}</div>
                <div className="mt-1 truncate text-[13px] text-[var(--muted)]">{e.subject}</div>
                {failed && e.error && <div className="mt-1 text-[12px] text-[var(--danger-fg)]">{e.error}</div>}
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
