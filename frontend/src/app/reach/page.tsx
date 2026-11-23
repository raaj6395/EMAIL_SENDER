"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  ReachContact,
  ReachPage,
  ReachProgress,
  Track,
  api,
} from "@/lib/api";
import { Alerts, Badge, Button, Card, EmptyRow, Notice, Page, SectionHeader, Toast } from "@/components/ui";
import { ContactRow, HRPager, RowList, SearchInput, SentSection } from "@/components/hr";
import { IconCheck } from "@/components/icons";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

const PAGE_SIZE = 25;

/** Comp band + confidence for a screened company. */
function ScreenBadge({ c }: { c: ReachContact }) {
  if (!c.screen) return <Badge>Not screened</Badge>;
  const { minLPA, maxLPA, confidence } = c.screen;
  const band = maxLPA > minLPA ? `${minLPA}–${maxLPA} LPA` : `${minLPA} LPA`;
  return (
    <>
      <span className="font-mono text-[13px] text-[var(--fg)]">₹{band}</span>
      {confidence === "low" && <Badge tone="warning">Low confidence</Badge>}
    </>
  );
}

/** Deliverability state for the chosen address. */
function VerifyBadge({ c }: { c: ReachContact }) {
  if (!c.verified) return <Badge>Email unchecked</Badge>;
  return c.verified.valid ? (
    <Badge tone="success">Email OK</Badge>
  ) : (
    <Badge tone="danger">{c.verified.code === "no_mx" ? "Dead domain" : "Bad address"}</Badge>
  );
}

/** One shortlisted contact: details, then Send now / Add to queue. */
function ReachRow({
  contact,
  busy,
  onSend,
  onQueue,
}: {
  contact: ReachContact;
  busy: "send" | "queue" | null;
  onSend: (c: ReachContact) => void;
  onQueue: (c: ReachContact) => void;
}) {
  return (
    <ContactRow
      actions={
        <>
          <Button variant="secondary" size="sm" onClick={() => onQueue(contact)} loading={busy === "queue"}>
            Add to queue
          </Button>
          <Button size="sm" onClick={() => onSend(contact)} loading={busy === "send"}>
            Send now
          </Button>
        </>
      }
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="text-[14px] font-medium">{contact.company}</span>
        <ScreenBadge c={contact} />
        <VerifyBadge c={contact} />
      </div>
      <div className="mt-1 truncate text-[13px] text-[var(--muted)]">
        {[contact.name, contact.location, contact.industry].filter(Boolean).join(" · ")}
      </div>
      <div className="mt-0.5 truncate font-mono text-[12px] text-[var(--subtle)]">{contact.email}</div>
      {contact.screen?.note && <p className="mt-1 text-[13px] leading-5 text-[var(--muted)]">{contact.screen.note}</p>}
    </ContactRow>
  );
}

/** One pipeline step: progress bar, count, and the button that advances it. */
function PipelineStep(p: {
  n: number;
  title: string;
  done: number;
  total: number;
  unit: string;
  pending: number;
  busy: boolean;
  action: string;
  onRun: () => void;
}) {
  const pct = p.total > 0 ? Math.round((p.done / p.total) * 100) : 0;
  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="flex items-baseline gap-2 text-[14px] font-medium">
            <span className="font-mono text-[12px] text-[var(--subtle)]">0{p.n}</span>
            {p.title}
          </div>
          <div className="mt-1 text-[13px] text-[var(--muted)]">
            {p.done.toLocaleString()} of {p.total.toLocaleString()} {p.unit}
            {p.pending === 0 && <IconCheck size={14} className="ml-1.5 inline text-[var(--success)]" />}
          </div>
        </div>
        <Button variant="secondary" size="sm" onClick={p.onRun} loading={p.busy} disabled={p.pending === 0}>
          {p.action}
        </Button>
      </div>
      <div className="mt-3 h-1 overflow-hidden rounded-full bg-[var(--surface-sunken)]">
        <div className="h-full rounded-full bg-[var(--fg)]/70 transition-transform duration-300" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

/** Pipeline progress and the two buttons that advance it. */
function PipelineBar({
  progress,
  verifying,
  screening,
  onVerify,
  onScreen,
}: {
  progress: ReachProgress | null;
  verifying: boolean;
  screening: boolean;
  onVerify: () => void;
  onScreen: () => void;
}) {
  if (!progress) return null;
  const { rows, companies, verified, pendingVerify, screened, pendingScreen } = progress;
  return (
    <div className="mb-6 grid gap-3 sm:grid-cols-2">
      <PipelineStep
        n={1}
        title="Verify emails"
        done={verified}
        total={rows}
        unit="contacts"
        pending={pendingVerify}
        busy={verifying}
        action={pendingVerify === 0 ? "All verified" : "Verify next"}
        onRun={onVerify}
      />
      <PipelineStep
        n={2}
        title="Screen companies"
        done={screened}
        total={companies}
        unit="companies"
        pending={pendingScreen}
        busy={screening}
        action={pendingScreen === 0 ? "All screened" : "Screen next"}
        onRun={onScreen}
      />
    </div>
  );
}

export default function ReachListPage() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [data, setData] = useState<ReachPage | null>(null);
  const [page, setPage] = useState(1);
  const [q, setQ] = useState("");
  const [showAll, setShowAll] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [screening, setScreening] = useState(false);
  const [busyRow, setBusyRow] = useState<{ email: string; kind: "send" | "queue" } | null>(null);
  const [toast, setToast] = useState<ToastState>(null);

  const errMsg = (e: unknown) =>
    e instanceof ApiError ? e.message : "Something went wrong. Please try again.";

  const load = useCallback(
    async (p: number, query: string, all: boolean) => {
      setLoading(true);
      setError(null);
      try {
        setData(await api.reach({ page: p, pageSize: PAGE_SIZE, q: query, all }));
        setPage(p);
      } catch (e) {
        setError(errMsg(e));
        setData(null);
      } finally {
        setLoading(false);
      }
    },
    []
  );

  useEffect(() => {
    api
      .health()
      .then((h) => setEnabled(h.reachEnabled))
      .catch(() => setEnabled(false));
  }, []);

  // Initial load + debounced reload when the search or the filter changes.
  useEffect(() => {
    const t = setTimeout(() => load(1, q, showAll), q ? 350 : 0);
    return () => clearTimeout(t);
  }, [q, showAll, load]);

  const runVerify = async () => {
    setVerifying(true);
    try {
      const r = await api.reachVerify();
      setToast({
        kind: "success",
        message: `Checked ${r.checked} addresses: ${r.valid} deliverable, ${r.bad} bad`,
      });
      await load(page, q, showAll);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setVerifying(false);
    }
  };

  const runScreen = async () => {
    setScreening(true);
    try {
      const r = await api.reachScreen();
      setToast({
        kind: "success",
        message: `Screened ${r.screened} companies. ${r.qualified} clear the 12+ LPA fresher bar`,
      });
      await load(page, q, showAll);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setScreening(false);
    }
  };

  // Send immediately: the same AI-tailored, resume-attached email the compose
  // page sends, then mark the contact so it drops off the list.
  const handleSend = async (c: ReachContact) => {
    if (!confirm(`Send your application email to ${c.name || c.company} <${c.email}> now?`)) return;
    setBusyRow({ email: c.email, kind: "send" });
    try {
      const track: Track = "sd";
      await api.send({
        recipientEmail: c.email,
        recipientName: c.name,
        company: c.company,
        track,
      });
      await api.hrMarkSent({
        channel: "reach",
        company: c.company,
        name: c.name,
        email: c.email,
        key: c.email,
      });
      setToast({ kind: "success", message: `Sent to ${c.email}` });
      await load(page, q, showAll);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setBusyRow(null);
    }
  };

  // Queue: goes out in the next configured send window instead of right now.
  const handleQueue = async (c: ReachContact) => {
    setBusyRow({ email: c.email, kind: "queue" });
    try {
      const r = await api.reachQueue(
        [{ email: c.email, company: c.company, name: c.name }],
        "sd"
      );
      setToast({
        kind: r.added > 0 ? "success" : "info",
        message:
          r.added > 0
            ? `${c.email} queued for the next send window`
            : `${c.email} was already in the queue`,
      });
      await load(page, q, showAll);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setBusyRow(null);
    }
  };

  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE));

  return (
    <Page>
      <SectionHeader
        title="Reach"
        subtitle="Startup contacts from reach.xlsx, narrowed to companies that hire fresher SDEs at 12+ LPA. The sheet has no role or pay data, so addresses are checked for delivery and companies are AI-screened. Run both steps, then work the list"
      />

      <Alerts>{toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}</Alerts>

      {enabled === false ? (
        <Notice tone="warning">
          No reach data found. Put your spreadsheet at <span className="font-mono">backend/data/reach.xlsx</span> and
          restart the backend
        </Notice>
      ) : (
        <Card title="Shortlist">
          <PipelineBar
            progress={data?.progress ?? null}
            verifying={verifying}
            screening={screening}
            onVerify={runVerify}
            onScreen={runScreen}
          />

          <div className="mb-4 flex flex-wrap items-center gap-3">
            <SearchInput value={q} onChange={setQ} placeholder="Search company, name, email or industry" />
            <span className="font-mono text-[12px] text-[var(--subtle)]">
              {loading ? "loading" : `${(data?.total ?? 0).toLocaleString()} contact${data?.total === 1 ? "" : "s"}`}
            </span>
            <label className="flex items-center gap-2 text-[13px] text-[var(--muted)]">
              <input
                type="checkbox"
                checked={showAll}
                onChange={(e) => setShowAll(e.target.checked)}
                className="h-4 w-4 accent-[var(--accent)]"
              />
              Include unscreened
            </label>
          </div>

          {error && (
            <Notice tone="danger" className="mb-4">
              {error}
            </Notice>
          )}

          <RowList>
            {(data?.contacts ?? []).map((c) => (
              <ReachRow
                key={c.email}
                contact={c}
                busy={busyRow?.email === c.email ? busyRow.kind : null}
                onSend={handleSend}
                onQueue={handleQueue}
              />
            ))}
          </RowList>
          {!loading && (data?.contacts.length ?? 0) === 0 && !error && (
            <EmptyRow>
              {data && data.progress.screened === 0
                ? "Nothing screened yet. Run Verify next and Screen next to build the shortlist"
                : "No contacts match. Try Include unscreened, or screen more companies"}
            </EmptyRow>
          )}

          <HRPager
            page={page}
            totalPages={totalPages}
            loading={loading}
            goto={(p) => load(Math.min(Math.max(1, p), totalPages), q, showAll)}
          />

          <SentSection sent={data?.sent ?? []} />
        </Card>
      )}

    </Page>
  );
}
