"use client";

import { useEffect, useState } from "react";
import { ApiError, InboxReply, api } from "@/lib/api";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  SectionHeader,
  Select,
  Textarea,
  Toast,
} from "@/components/ui";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}
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
    <div className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--elevated)] p-4 shadow-[var(--shadow-sm)]">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium text-[var(--fg)]">{reply.fromName || reply.fromEmail}</span>
            <Badge tone={categoryTone[reply.category]}>{categoryLabel[reply.category]}</Badge>
          </div>
          <div className="mt-0.5 truncate text-xs text-[var(--muted)]">
            {reply.subject} · {reply.fromEmail}
          </div>
        </div>
        <span className="shrink-0 text-xs text-[var(--muted)]">{formatWhen(reply.receivedAt)}</span>
      </div>

      {reply.reason && <p className="mt-2 text-xs text-[var(--muted)]"><span className="font-medium">AI:</span> {reply.reason}</p>}

      <button
        onClick={() => setShowMsg((v) => !v)}
        className="mt-2 text-xs font-medium text-[var(--accent)] hover:underline"
      >
        {showMsg ? "Hide their message" : "Show their message"}
      </button>
      {showMsg && (
        <pre className="mt-2 max-h-40 overflow-y-auto whitespace-pre-wrap rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-3 text-xs text-[var(--muted)]">
          {reply.body || "(no text content)"}
        </pre>
      )}

      <div className="mt-3">
        <label className="mb-1.5 block text-[13px] font-medium text-[var(--fg)]">Your reply</label>
        <Textarea value={draft} onChange={(e) => setDraft(e.target.value)} rows={5} />
      </div>

      <div className="mt-3 flex items-center gap-2">
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
        message: `Scanned ${r.checked} recent emails — ${r.matched} were replies to your outreach.`,
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
      setToast({ kind: "success", message: "Reply sent ✓" });
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

  return (
    <main className="mx-auto w-full max-w-4xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
      <SectionHeader
        title="Replies"
        subtitle="Reads recent replies to your outreach and drafts responses with AI. Rejections are set aside; “check back later” replies become follow-up reminders."
        action={
          <div className="flex items-center gap-2">
            <Select value={limit} onChange={(e) => setLimit(Number(e.target.value))} disabled={checking} className="w-auto">
              <option value={20}>Last 20</option>
              <option value={40}>Last 40</option>
              <option value={60}>Last 60</option>
            </Select>
            <Button onClick={check} loading={checking} disabled={inboxEnabled === false}>
              {checking ? "Checking…" : "Check inbox"}
            </Button>
          </div>
        }
      />

      {toast && (
        <div className="mb-5">
          <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />
        </div>
      )}

      {inboxEnabled === false ? (
        <Card>
          <EmptyState icon="🔒" title="Inbox reading needs your Gmail credentials">
            Set <span className="font-mono">GMAIL_USER</span> and{" "}
            <span className="font-mono">GMAIL_APP_PASSWORD</span> in{" "}
            <span className="font-mono">backend/.env</span>, then restart the backend. The same App
            Password used for sending also reads your inbox (read-only).
          </EmptyState>
        </Card>
      ) : (
        <div className="space-y-6">
          {/* Needs reply */}
          <div>
            <h2 className="mb-2 text-[13px] font-semibold uppercase tracking-wide text-[var(--muted)]">
              Needs reply{needsReply.length > 0 && ` · ${needsReply.length}`}
            </h2>
            {needsReply.length === 0 ? (
              <EmptyState icon="✅" title="No replies waiting">
                Click <span className="font-medium">Check inbox</span> to scan for new replies to your
                outreach.
              </EmptyState>
            ) : (
              <div className="space-y-3">
                {needsReply.map((r) => (
                  <NeedsReplyCard key={r.messageId} reply={r} onSend={handleSend} onDismiss={handleDismiss} />
                ))}
              </div>
            )}
          </div>

          {/* Scheduled follow-ups */}
          {scheduled.length > 0 && (
            <div>
              <h2 className="mb-2 text-[13px] font-semibold uppercase tracking-wide text-[var(--muted)]">
                Follow up later · {scheduled.length}
              </h2>
              <div className="space-y-2">
                {scheduled.map((r) => (
                  <div
                    key={r.messageId}
                    className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 text-sm"
                  >
                    <div className="min-w-0">
                      <span className="font-medium">{r.fromName || r.fromEmail}</span>
                      <span className="ml-2 text-xs text-[var(--muted)]">{r.reason}</span>
                    </div>
                    {r.followUpAt && <Badge tone="warning">Follow up {formatDay(r.followUpAt)}</Badge>}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Replied */}
          {replied.length > 0 && (
            <div>
              <h2 className="mb-2 text-[13px] font-semibold uppercase tracking-wide text-[var(--muted)]">
                Replied · {replied.length}
              </h2>
              <div className="space-y-2">
                {replied.map((r) => (
                  <div
                    key={r.messageId}
                    className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 text-sm"
                  >
                    <span className="min-w-0 truncate">
                      <span className="font-medium">{r.fromName || r.fromEmail}</span>
                      <span className="ml-2 text-xs text-[var(--muted)]">{r.subject}</span>
                    </span>
                    <Badge tone="success">Replied</Badge>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* No action */}
          {noAction.length > 0 && (
            <div>
              <h2 className="mb-2 text-[13px] font-semibold uppercase tracking-wide text-[var(--muted)]">
                No action · {noAction.length}
              </h2>
              <div className="space-y-2">
                {noAction.map((r) => (
                  <div
                    key={r.messageId}
                    className="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 text-sm opacity-75"
                  >
                    <span className="min-w-0 truncate">
                      <span className="font-medium">{r.fromName || r.fromEmail}</span>
                      <span className="ml-2 text-xs text-[var(--muted)]">{r.reason}</span>
                    </span>
                    <Badge tone={r.status === "dismissed" ? "neutral" : "danger"}>
                      {r.status === "dismissed" ? "Dismissed" : "Not open"}
                    </Badge>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </main>
  );
}
