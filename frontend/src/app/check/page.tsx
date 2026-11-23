"use client";

import { useRef, useState } from "react";
import { ApiError, ExistsLine, Track, api } from "@/lib/api";
import { Alerts, Button, Card, Page, SectionHeader, Segmented, Textarea, Toast } from "@/components/ui";
import { TRACK_OPTIONS } from "@/components/ComposeForm";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

type Found = { email: string; company?: string; queued: boolean };

type Live = {
  total: number;
  checked: number;
  exist: number;
  invalid: number;
  unknown: number;
  added: number;
  emails: Found[];
  done: boolean;
};

function countLines(text: string): number {
  return text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l && /\S+@\S+\.\S+/.test(l.split(/[,\t]/)[0].trim())).length;
}

const emptyLive = (total: number): Live => ({
  total,
  checked: 0,
  exist: 0,
  invalid: 0,
  unknown: 0,
  added: 0,
  emails: [],
  done: false,
});

export default function CheckPage() {
  const [rows, setRows] = useState("");
  const [track, setTrack] = useState<Track>("sd");
  const [live, setLive] = useState<Live | null>(null);
  const [checking, setChecking] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const abortRef = useRef<AbortController | null>(null);

  const n = countLines(rows);

  const onLine = (line: ExistsLine) => {
    if (line.error) {
      setToast({ kind: "error", message: line.error });
      return;
    }
    if (line.done) {
      setLive((cur) => (cur ? { ...cur, done: true } : cur));
      return;
    }
    if (!line.kind) return;
    setLive((cur) => {
      if (!cur) return cur;
      const next = { ...cur, checked: cur.checked + 1 };
      if (line.kind === "exist") {
        next.exist += 1;
        if (line.queued) next.added += 1;
        next.emails = [
          ...cur.emails,
          { email: line.email ?? "", company: line.company, queued: !!line.queued },
        ];
      } else if (line.kind === "invalid") {
        next.invalid += 1;
      } else {
        next.unknown += 1;
      }
      return next;
    });
  };

  const check = async () => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setChecking(true);
    setToast(null);
    setLive(emptyLive(n));
    try {
      await api.checkExists(rows, track, onLine, ac.signal);
    } catch (e) {
      if (ac.signal.aborted) return;
      setToast({
        kind: "error",
        message: e instanceof ApiError ? e.message : "Could not check these emails.",
      });
    } finally {
      if (!ac.signal.aborted) {
        setChecking(false);
        setLive((cur) => (cur && !cur.done ? { ...cur, done: true } : cur));
      }
    }
  };

  const profile = track === "ai" ? "AI" : "SDE";

  return (
    <Page>
      <SectionHeader
        title="Check emails"
        subtitle="Asks each mail server whether an address exists. Confirmed ones join the bulk queue as soon as they pass, so a long list never has to finish first"
      />

      <Alerts>{toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}</Alerts>

      <Card title="Addresses">
        <Textarea
          value={rows}
          onChange={(e) => setRows(e.target.value)}
          rows={8}
          placeholder={"priya@stripe.com\nraj@carousell.com, Carousell, Raj\nhr@openai.com, OpenAI"}
          className="font-mono text-[13px]"
          disabled={checking}
        />
        <p className="mt-2 text-[12px] leading-4 text-[var(--subtle)]">
          One per line, or <span className="font-mono">email, Company, Name</span>. Gmail and Outlook often refuse to
          answer, and those are not queued
        </p>
        <div className="mt-5 flex flex-wrap items-center gap-3">
          <Segmented value={track} options={TRACK_OPTIONS} onChange={setTrack} disabled={checking} />
          <Button onClick={check} disabled={n === 0} loading={checking}>
            {checking && live
              ? `Checking ${live.checked} of ${live.total}`
              : n
                ? `Check ${n}`
                : "Check"}
          </Button>
        </div>
      </Card>

      {live && (
        <div className="mt-6">
          <Card
            title={
              live.done
                ? `${live.exist} of ${live.checked} exist`
                : `Checking ${live.checked} of ${live.total}`
            }
            action={
              <span className="font-mono text-[12px] text-[var(--subtle)]">
                {live.added} queued
                {live.invalid > 0 ? ` · ${live.invalid} rejected` : ""}
                {live.unknown > 0 ? ` · ${live.unknown} unconfirmed` : ""}
              </span>
            }
          >
            {live.emails.length === 0 ? (
              <p className="text-[13px] text-[var(--muted)]">
                {live.done
                  ? "None of these came back as a real mailbox"
                  : "Confirmed addresses show up here and join the bulk queue as each check finishes"}
              </p>
            ) : (
              <>
                <ul className="max-h-80 overflow-auto rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)]">
                  {live.emails.map((e) => (
                    <li key={e.email} className="flex h-9 items-center gap-3 border-b border-[var(--border)] px-3 last:border-b-0">
                      <span className="truncate font-mono text-[12px] text-[var(--fg)]">{e.email}</span>
                      {e.company && <span className="truncate text-[13px] text-[var(--muted)]">{e.company}</span>}
                      <span className="ml-auto shrink-0 text-[12px] text-[var(--subtle)]">
                        {e.queued ? "Queued" : "Already queued"}
                      </span>
                    </li>
                  ))}
                </ul>
                {live.done && live.added > 0 && (
                  <p className="mt-3 text-[13px] text-[var(--muted)]">
                    {live.added} added to the {profile} bulk queue. They go out in the next send window
                  </p>
                )}
              </>
            )}
          </Card>
        </div>
      )}
    </Page>
  );
}
