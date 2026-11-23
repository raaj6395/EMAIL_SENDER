"use client";

import { useEffect, useRef, useState } from "react";
import { ApiError, BatchStatus, Track, api, formatCountdown } from "@/lib/api";
import { Button, Card, Dot, Notice, Postmark, Textarea } from "./ui";

/** Count non-empty lines that look like they contain an email. */
function countRecipients(text: string): number {
  return text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l && /\S+@\S+\.\S+/.test(l.split(/[,\t]/)[0].trim())).length;
}

type ItemTone = "success" | "danger" | "info" | "neutral";
function statusTone(s: string): ItemTone {
  if (s === "sent") return "success";
  if (s === "failed") return "danger";
  if (s === "sending") return "info";
  return "neutral";
}

/** Format an RFC3339 instant as "Mon 11:00" in the viewer's locale. */
function whenLabel(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString(undefined, {
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** One line on what the queue is doing right now. */
function QueueBanner({ status }: { status: BatchStatus }) {
  if (status.sendNow && status.remaining > 0) {
    return (
      <Notice tone="accent">
        Sending now, ignoring the schedule and window limit for the {status.remaining} still waiting
        {status.nextInSec > 0 && <>. Next email in about {status.nextInSec}s</>}
      </Notice>
    );
  }
  if (status.paused) {
    return <Notice tone="warning">Queue paused. Nothing sends, even inside a window, until you resume it</Notice>;
  }
  if (status.remaining === 0) return null;
  if (status.capReached) {
    return (
      <Notice tone="info">
        Window limit reached ({status.sentInWindow} of {status.perWindowCap}). The other {status.remaining} go out{" "}
        {whenLabel(status.nextWindowAt)}
      </Notice>
    );
  }
  if (status.sending) {
    return (
      <Notice tone="success">
        Window open, sending now
        {status.nextInSec > 0 && <>. Next email in about {status.nextInSec}s</>}
        {status.windowEndsAt && <>. Closes at {whenLabel(status.windowEndsAt)}</>}
      </Notice>
    );
  }
  return (
    <Notice>
      {status.remaining} waiting. The next window opens{" "}
      <span className="text-[var(--fg)]">{whenLabel(status.nextWindowAt)}</span>
      {status.opensInSec > 0 && <>, in {formatCountdown(status.opensInSec)}</>}
    </Notice>
  );
}

export function BulkSend({ track }: { track: Track }) {
  const [rows, setRows] = useState("");
  const [status, setStatus] = useState<BatchStatus | null>(null);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Poll the queue. Fast while actively sending, slow while waiting for a
  // window to open (nothing changes for hours). State is only set from the
  // fetch callback, never synchronously in the effect body.
  const sending = status?.sending ?? false;
  useEffect(() => {
    let alive = true;
    const fetchOnce = () => {
      api
        .queueStatus()
        .then((s) => {
          if (alive) setStatus(s);
        })
        .catch(() => {
          /* transient — the next tick retries */
        });
    };
    fetchOnce();
    pollRef.current = setInterval(fetchOnce, sending ? 3000 : 30000);
    return () => {
      alive = false;
      if (pollRef.current) clearInterval(pollRef.current);
      pollRef.current = null;
    };
  }, [sending]);

  const recipientCount = countRecipients(rows);
  const paused = status?.paused ?? false;

  const handleAdd = async () => {
    setAdding(true);
    setError(null);
    setNote(null);
    try {
      const res = await api.queueAdd(rows, track);
      setStatus(res.queue);
      setRows("");
      const parts = [`Queued ${res.added} recipient${res.added === 1 ? "" : "s"}`];
      if (res.skipped > 0) parts.push(`${res.skipped} already in the queue`);
      setNote(parts.join(" · "));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not queue these recipients.");
    } finally {
      setAdding(false);
    }
  };

  const act = async (fn: () => Promise<BatchStatus>) => {
    try {
      setStatus(await fn());
    } catch {
      /* ignore — the next poll will resync */
    }
  };

  return (
    <Card
      title={`Bulk queue · ${track === "ai" ? "AI" : "SDE"} profile`}
      collapsible
      defaultOpen={false}
      action={
        status && status.remaining > 0 ? (
          <span className="font-mono text-[12px] text-[var(--subtle)]">{status.remaining} waiting</span>
        ) : null
      }
    >
      <p className="mb-3 text-[13px] leading-5 text-[var(--muted)]">
        One recipient per line, as an email or <span className="font-mono text-[12px] text-[var(--fg)]">email, Company, Name</span>.
        Missing companies are guessed from the domain. Each gets an AI-tailored email with your{" "}
        {track === "ai" ? "AI" : "SDE"} resume during the next send window.
      </p>

      <Textarea
        value={rows}
        onChange={(e) => setRows(e.target.value)}
        rows={6}
        placeholder={"priya@stripe.com\nraj@carousell.com, Carousell, Raj\nhr@openai.com, OpenAI"}
        className="font-mono text-[13px]"
      />

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Button onClick={handleAdd} disabled={recipientCount === 0} loading={adding}>
          {`Add ${recipientCount || 0} to queue`}
        </Button>
        {status && status.remaining > 0 && !status.sendNow && (
          <Button
            variant="secondary"
            onClick={async () => {
              setError(null);
              try {
                const res = await api.queueSendNow();
                setStatus(res.queue);
                setNote(
                  `Sending ${res.sending} now, outside the schedule and window limit`
                );
              } catch (e) {
                setError(e instanceof ApiError ? e.message : "Could not start sending.");
              }
            }}
            title="Send everything waiting right now, ignoring the Mon/Wed windows"
          >
            Send now
          </Button>
        )}
        {status?.sendNow && (
          <Button
            variant="secondary"
            onClick={async () => {
              setStatus(await api.queueSendNowCancel());
              setNote(null);
            }}
          >
            Back to schedule
          </Button>
        )}
        {status && status.failed > 0 && (
          <Button
            variant="secondary"
            onClick={async () => {
              const res = await api.queueRetryFailed();
              setStatus(res.queue);
              setNote(`Re-queued ${res.requeued} failed recipient${res.requeued === 1 ? "" : "s"}`);
            }}
            title="Failed sends never reached the recipient, so retrying cannot duplicate an email"
          >
            Retry {status.failed} failed
          </Button>
        )}
        {status && status.remaining > 0 && !paused && (
          <Button variant="secondary" onClick={() => act(api.queuePause)}>
            Pause queue
          </Button>
        )}
        {paused && (
          <Button variant="secondary" onClick={() => act(api.queueResume)}>
            Resume queue
          </Button>
        )}
        {status && status.remaining > 0 && (
          <Button variant="danger" onClick={() => act(api.queueClearWaiting)}>
            Clear waiting
          </Button>
        )}
      </div>

      {note && <p className="mt-3 text-[13px] text-[var(--success-fg)]">{note}</p>}

      {error && (
        <Notice tone="danger" className="mt-3">
          {error}
        </Notice>
      )}

      {status && status.total > 0 && (
        <div className="mt-5 space-y-3">
          <QueueBanner status={status} />

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-[var(--muted)]">
            <span>
              <span className="font-medium text-[var(--accent)]">{status.sent}</span> sent
            </span>
            {status.failed > 0 && (
              <span>
                <span className="font-medium text-[var(--danger-fg)]">{status.failed}</span> failed
              </span>
            )}
            <span>{status.remaining} waiting</span>
            <span>{status.total} total</span>
            {status.perWindowCap > 0 && status.windowOpen && (
              <span>
                {status.sentInWindow} of {status.perWindowCap} this window
              </span>
            )}
            {status.total > 0 && status.remaining === 0 && (
              <button
                onClick={() => act(api.queueClearAll)}
                className="ml-auto text-[12px] text-[var(--subtle)] underline underline-offset-2 hover:text-[var(--fg)]"
              >
                Clear list
              </button>
            )}
          </div>

          <div className="max-h-72 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)]">
            {status.items.map((it, i) => (
              <div
                key={`${it.email}-${i}`}
                className="flex h-9 items-center gap-2.5 border-b border-[var(--border)] px-3 text-[13px] last:border-b-0"
              >
                <Dot tone={statusTone(it.status)} />
                <span className="truncate font-mono text-[12px] text-[var(--fg)]">{it.email}</span>
                <span className="truncate text-[var(--muted)]">
                  {it.company}
                  {it.error ? ` · ${it.error}` : ""}
                </span>
                <span className="ml-auto">
                  {it.status === "sent" && it.sentAt ? (
                    <Postmark iso={it.sentAt} label={whenLabel(it.sentAt)} sent />
                  ) : (
                    <span className="text-[12px] text-[var(--subtle)]">{it.status}</span>
                  )}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}
