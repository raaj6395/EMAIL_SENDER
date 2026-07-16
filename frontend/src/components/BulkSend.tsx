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
      return "bg-green-500";
    case "failed":
      return "bg-red-500";
    case "sending":
      return "bg-blue-500 animate-pulse";
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

  const handleCancel = async () => {
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

      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Button onClick={handleStart} disabled={recipientCount === 0 || running} loading={starting}>
          {running ? "Sending…" : `Send to ${recipientCount || 0} recipient${recipientCount === 1 ? "" : "s"}`}
        </Button>
        {running && (
          <Button variant="secondary" onClick={handleCancel}>
            Cancel
          </Button>
        )}
        {!running && recipientCount > 0 && (
          <span className="text-xs text-[var(--muted)]">
            Sent one at a time with a random 0–20s gap to protect your Gmail account.
          </span>
        )}
      </div>

      {error && (
        <div className="mt-3 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-700 dark:text-red-300">
          {error}
        </div>
      )}

      {status && status.total > 0 && (
        <div className="mt-4">
          <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <span>
              <strong className="text-green-600 dark:text-green-400">{status.sent}</strong> sent
            </span>
            {status.failed > 0 && (
              <span>
                <strong className="text-red-500">{status.failed}</strong> failed
              </span>
            )}
            <span className="text-[var(--muted)]">{status.remaining} remaining</span>
            <span className="text-[var(--muted)]">· {status.total} total</span>
            {status.active && status.nextInSec > 0 && (
              <span className="text-[var(--muted)]">· next in ~{status.nextInSec}s</span>
            )}
            {status.done && <span className="font-medium text-[var(--accent)]">· done</span>}
          </div>

          <div className="max-h-64 space-y-1 overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--background)] p-2">
            {status.items.map((it, i) => (
              <div key={`${it.email}-${i}`} className="flex items-center gap-2 text-xs">
                <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${statusColor(it.status)}`} />
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
