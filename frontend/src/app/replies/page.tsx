"use client";

import { useEffect, useState } from "react";
import { ApiError, InboxReply, api } from "@/lib/api";
import {
  Alerts,
  Badge,
  Button,
  Card,
  EmptyState,
  Page,
  Postmark,
  SectionHeader,
  Select,
  SubHeading,
  Textarea,
  Toast,
} from "@/components/ui";
import { RowList } from "@/components/hr";
import { IconInbox, IconLock } from "@/components/icons";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

function formatDay(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

const categoryTone: Record<InboxReply["category"], "success" | "info" | "warning" | "danger" | "neutral"> = {
  positive: "success",
  question: "info",
  follow_up_later: "warning",
  not_open: "danger",
  other: "neutral",
};
const categoryLabel: Record<InboxReply["category"], string> = {
  positive: "Interested",
  question: "Question",
  follow_up_later: "Later",
  not_open: "Not open",
  other: "Other",
};

/** A card for a reply that needs a response, with an editable AI draft. */
function NeedsReplyCard({
  reply,
  onSend,
  onDismiss,
}: {
  reply: InboxReply;
  onSend: (messageId: string, body: string) => Promise<void>;
  onDismiss: (messageId: string) => void;
}) {
  const [draft, setDraft] = useState(reply.draft);
  const [showMsg, setShowMsg] = useState(false);
  const [sending, setSending] = useState(false);

  const send = async () => {
    setSending(true);
    try {
      await onSend(reply.messageId, draft);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--elevated)] p-5 shadow-[var(--shadow-sm)]">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[14px] font-medium text-[var(--fg)]">{reply.fromName || reply.fromEmail}</span>
            <Badge tone={categoryTone[reply.category]}>{categoryLabel[reply.category]}</Badge>
          </div>
          <div className="mt-0.5 truncate text-[13px] text-[var(--muted)]">
            {reply.subject} · <span className="font-mono text-[12px]">{reply.fromEmail}</span>
          </div>
        </div>
        <Postmark iso={reply.receivedAt} />
      </div>

      {reply.reason && <p className="mt-3 text-[13px] leading-5 text-[var(--muted)]">{reply.reason}</p>}

      <button
        onClick={() => setShowMsg((v) => !v)}
        className="mt-2 text-[13px] text-[var(--fg)] underline decoration-[var(--border-strong)] underline-offset-4 hover:decoration-[var(--fg)]"
      >
        {showMsg ? "Hide their message" : "Show their message"}
      </button>
      {showMsg && (
        <pre className="mt-2 max-h-48 overflow-y-auto whitespace-pre-wrap rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-3 font-sans text-[13px] leading-5 text-[var(--muted)]">
          {reply.body || "No text content"}
        </pre>
      )}

      <div className="mt-4">
        <label className="mb-2 block text-[13px] font-medium text-[var(--fg)]">Your reply</label>
        <Textarea value={draft} onChange={(e) => setDraft(e.target.value)} rows={5} />
      </div>

      <div className="mt-4 flex items-center gap-2">
        <Button onClick={send} loading={sending} disabled={!draft.trim()}>
          Send reply
        </Button>
        <Button variant="ghost" onClick={() => onDismiss(reply.messageId)}>
          Dismiss
        </Button>
      </div>
    </div>
  );
}

export default function RepliesPage() {
  const [inboxEnabled, setInboxEnabled] = useState<boolean | null>(null);
  const [replies, setReplies] = useState<InboxReply[]>([]);
  const [limit, setLimit] = useState(40);
  const [checking, setChecking] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);

  const errMsg = (e: unknown) =>
    e instanceof ApiError ? e.message : "Something went wrong. Please try again.";

  useEffect(() => {
    api.health().then((h) => setInboxEnabled(h.inboxEnabled)).catch(() => setInboxEnabled(false));
    api.replies().then((r) => setReplies(r.replies ?? [])).catch(() => {});
  }, []);

  const check = async () => {
    setChecking(true);
    setToast(null);
    try {
      const r = await api.repliesCheck(limit);
      setReplies(r.replies ?? []);
      setToast({
        kind: "info",
        message: `Scanned ${r.checked} recent emails. ${r.matched} were replies to your outreach`,
      });
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setChecking(false);
    }
  };

  const handleSend = async (messageId: string, body: string) => {
    try {
      const r = await api.replySend(messageId, body);
      setReplies(r.replies ?? []);
      setToast({ kind: "success", message: "Reply sent" });
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    }
  };

  const handleDismiss = async (messageId: string) => {
    try {
      const r = await api.replyDismiss(messageId);
      setReplies(r.replies ?? []);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    }
  };

  const needsReply = replies.filter((r) => r.status === "needs_reply");
  const scheduled = replies.filter((r) => r.status === "scheduled");
  const noAction = replies.filter((r) => r.status === "no_action" || r.status === "dismissed");
  const replied = replies.filter((r) => r.status === "replied");

  const quietRow = (r: InboxReply, note: string, tag: React.ReactNode) => (
    <div key={r.messageId} className="flex items-center justify-between gap-4 px-4 py-3">
      <div className="min-w-0">
        <div className="truncate text-[14px] font-medium">{r.fromName || r.fromEmail}</div>
        {note && <div className="mt-0.5 truncate text-[13px] text-[var(--muted)]">{note}</div>}
      </div>
      {tag}
    </div>
  );

  return (
    <Page>
      <SectionHeader
        title="Replies"
        subtitle="Reads recent replies to your outreach and drafts a response for each. Rejections are set aside, and check-back-later replies become follow-up reminders"
        action={
          <>
            <Select value={limit} onChange={(e) => setLimit(Number(e.target.value))} disabled={checking} className="w-auto">
              <option value={20}>Last 20</option>
              <option value={40}>Last 40</option>
              <option value={60}>Last 60</option>
            </Select>
            <Button onClick={check} loading={checking} disabled={inboxEnabled === false}>
              {checking ? "Checking" : "Check inbox"}
            </Button>
          </>
        }
      />

      <Alerts>{toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}</Alerts>

      {inboxEnabled === false ? (
        <Card>
          <EmptyState icon={<IconLock size={20} />} title="Reading the inbox needs your Gmail credentials">
            Set <span className="font-mono">GMAIL_USER</span> and <span className="font-mono">GMAIL_APP_PASSWORD</span> in{" "}
            <span className="font-mono">backend/.env</span>, then restart the backend. The App Password used for sending
            also reads your inbox, read-only
          </EmptyState>
        </Card>
      ) : (
        <div className="space-y-8">
          <section>
            <SubHeading count={needsReply.length}>Needs reply</SubHeading>
            {needsReply.length === 0 ? (
              <EmptyState icon={<IconInbox size={20} />} title="No replies waiting">
                Check inbox scans for new replies to your outreach
              </EmptyState>
            ) : (
              <div className="space-y-4">
                {needsReply.map((r) => (
                  <NeedsReplyCard key={r.messageId} reply={r} onSend={handleSend} onDismiss={handleDismiss} />
                ))}
              </div>
            )}
          </section>

          {scheduled.length > 0 && (
            <section>
              <SubHeading count={scheduled.length}>Follow up later</SubHeading>
              <RowList>
                {scheduled.map((r) =>
                  quietRow(r, r.reason, r.followUpAt && <Postmark iso={r.followUpAt} label={formatDay(r.followUpAt)} />)
                )}
              </RowList>
            </section>
          )}

          {replied.length > 0 && (
            <section>
              <SubHeading count={replied.length}>Replied</SubHeading>
              <RowList>{replied.map((r) => quietRow(r, r.subject, <Badge tone="success">Replied</Badge>))}</RowList>
            </section>
          )}

          {noAction.length > 0 && (
            <section>
              <SubHeading count={noAction.length}>No action</SubHeading>
              <RowList>
                {noAction.map((r) =>
                  quietRow(r, r.reason, <Badge>{r.status === "dismissed" ? "Dismissed" : "Not open"}</Badge>)
                )}
              </RowList>
            </section>
          )}
        </div>
      )}
    </Page>
  );
}
