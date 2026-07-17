"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, BatchStatus, Track, api } from "@/lib/api";
import { Button, Card, Textarea } from "./ui";

/** Count non-empty lines that look like they contain an email. */
function countRecipients(text: string): number {
  return text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l && /\S+@\S+\.\S+/.test(l.split(/[,\t]/)[0].trim())).length;
}

function statusColor(s: string): string {
  switch (s) {
    case "sent":
      return "bg-[var(--success)]";
    case "failed":
      return "bg-[var(--danger)]";
    case "sending":
      return "bg-[var(--info)] animate-pulse";
    case "skipped":
      return "bg-[var(--muted)]";
    default:
      return "bg-[var(--border)]";
  }
}

export function BulkSend({ track }: { track: Track }) {
  const [rows, setRows] = useState("");
  const [status, setStatus] = useState<BatchStatus | null>(null);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const poll = useCallback(async () => {
    try {
      const s = await api.batchStatus();
      setStatus(s);
      if (!s.active) stopPolling();
    } catch {
      /* transient — keep polling */
    }
  }, [stopPolling]);

  // On mount, pick up any batch already running (survives navigation/reload).
  useEffect(() => {
    api
      .batchStatus()
      .then((s) => {
        setStatus(s);
        if (s.active && !pollRef.current) pollRef.current = setInterval(poll, 2000);
      })
      .catch(() => {});
    return stopPolling;
  }, [poll, stopPolling]);

  const recipientCount = countRecipients(rows);
  const running = status?.active ?? false;
  const paused = status?.paused ?? false;

  const handleStart = async () => {
    setStarting(true);
    setError(null);
    try {
      const s = await api.batchStart(rows, track);
      setStatus(s);
      if (!pollRef.current) pollRef.current = setInterval(poll, 2000);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not start the bulk send.");
    } finally {
      setStarting(false);
    }
  };

  const handlePause = async () => {
    try {
      setStatus(await api.batchPause());
    } catch {
      /* ignore */
    }
  };

  const handleResume = async () => {
    try {
      const s = await api.batchResume();
      setStatus(s);
      if (s.active && !pollRef.current) pollRef.current = setInterval(poll, 2000);
    } catch {
      /* ignore */
    }
  };

  const handleAbort = async () => {
    try {
      setStatus(await api.batchCancel());
    } catch {
      /* ignore */
    }
  };

  return (
    <Card
      title={`Bulk send — ${track === "ai" ? "AI" : "SDE"} profile`}
      collapsible
      defaultOpen={false}
    >
      <p className="mb-2 text-sm text-[var(--muted)]">
        Paste one recipient per line — just an email, or{" "}
        <span className="font-mono text-xs">email, Company, Name</span>. Missing companies are guessed
        from the domain. Each gets an AI-tailored email with your{" "}
        <strong>{track === "ai" ? "AI" : "SDE"}</strong> resume.
      </p>

      <Textarea
        value={rows}
        onChange={(e) => setRows(e.target.value)}
        rows={6}
        placeholder={"priya@stripe.com\nraj@carousell.com, Carousell, Raj\nhr@openai.com, OpenAI"}
        disabled={running}
        className="font-mono text-xs"
      />

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Button onClick={handleStart} disabled={recipientCount === 0 || running} loading={starting}>
          {running
            ? paused
              ? "Paused"
              : "Sending…"
            : `Send to ${recipientCount || 0} recipient${recipientCount === 1 ? "" : "s"}`}
        </Button>
        {running && !paused && (
          <Button variant="secondary" onClick={handlePause}>
            Stop
          </Button>
        )}
        {running && paused && (
          <Button variant="secondary" onClick={handleResume}>
            Resume
          </Button>
        )}
        {running && (
          <Button variant="danger" onClick={handleAbort}>
            Abort
          </Button>
        )}
        {!running && recipientCount > 0 && (
          <span className="text-xs text-[var(--muted)]">
            Sent one at a time with a random 0–20s gap to protect your Gmail account.
          </span>
        )}
      </div>

      {error && (
        <div className="mt-3 rounded-[var(--radius-md)] border border-[var(--danger)]/40 bg-[var(--danger-soft)] px-3 py-2 text-sm text-[var(--danger-fg)]">
          {error}
        </div>
      )}

      {status && status.total > 0 && (
        <div className="mt-4">
          <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <span>
              <strong className="text-[var(--success-fg)]">{status.sent}</strong> sent
            </span>
            {status.failed > 0 && (
              <span>
                <strong className="text-[var(--danger-fg)]">{status.failed}</strong> failed
              </span>
            )}
            <span className="text-[var(--muted)]">{status.remaining} remaining</span>
            <span className="text-[var(--muted)]">· {status.total} total</span>
            {status.paused ? (
              <span className="font-medium text-[var(--warning-fg)]">· paused</span>
            ) : (
              status.active &&
              status.nextInSec > 0 && (
                <span className="text-[var(--muted)]">· next in ~{status.nextInSec}s</span>
              )
            )}
            {status.done && <span className="font-medium text-[var(--accent)]">· done</span>}
          </div>

          <div className="max-h-64 space-y-1 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-2">
            {status.items.map((it, i) => (
              <div key={`${it.email}-${i}`} className="flex items-center gap-2 text-xs">
                <span className={`inline-block h-2 w-2 shrink-0 rounded-[var(--radius-full)] ${statusColor(it.status)}`} />
                <span className="truncate font-medium">{it.email}</span>
                <span className="truncate text-[var(--muted)]">
                  {it.company}
                  {it.error ? ` — ${it.error}` : ""}
                </span>
                <span className="ml-auto shrink-0 text-[var(--muted)]">{it.status}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}
