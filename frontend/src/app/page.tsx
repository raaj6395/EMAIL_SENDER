"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiError,
  ComposeInput,
  ComposeResult,
  Health,
  HistoryEntry,
  MarkSentInput,
  Profile,
  Track,
  api,
  emptyProfile,
  takeComposePrefill,
} from "@/lib/api";
import { SetupBanner } from "@/components/SetupBanner";
import { ProfileEditor } from "@/components/ProfileEditor";
import { LinkedInLookup, LookupFound } from "@/components/LinkedInLookup";
import { ComposeForm } from "@/components/ComposeForm";
import { BulkSend } from "@/components/BulkSend";
import { EmailPreview } from "@/components/EmailPreview";
import { SendHistory } from "@/components/SendHistory";
import { StatusPanel } from "@/components/StatusPanel";
import { Toast } from "@/components/ui";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

/** Small status pill for the top bar. */
function Pill({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ${
        ok
          ? "border-green-500/30 bg-green-500/10 text-green-700 dark:text-green-300"
          : "border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-300"
      }`}
    >
      <span className={`inline-block h-1.5 w-1.5 rounded-full ${ok ? "bg-green-500" : "bg-red-500"}`} />
      {label}
    </span>
  );
}

export default function Home() {
  const [health, setHealth] = useState<Health | null>(null);
  const [track, setTrack] = useState<Track>("sd");
  const [profile, setProfile] = useState<Profile>(emptyProfile());
  const [compose, setCompose] = useState<ComposeInput>({ recipientEmail: "", recipientName: "", company: "", role: "" });
  const [rendered, setRendered] = useState<ComposeResult | null>(null);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  // When composing to a contact from the Email HR page, this marks them "sent"
  // after the email actually goes out.
  const [pendingHRSent, setPendingHRSent] = useState<MarkSentInput | null>(null);

  const [parsing, setParsing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [sending, setSending] = useState(false);
  const [digesting, setDigesting] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);

  const errMsg = (e: unknown) =>
    e instanceof ApiError ? e.message : "Something went wrong. Please try again.";

  const refreshHealth = useCallback(async () => {
    try {
      setHealth(await api.health());
    } catch {
      setHealth(null);
      setToast({
        kind: "error",
        message:
          "Cannot reach the backend at http://localhost:8080. Start it with: cd backend && go run .",
      });
    }
  }, []);

  const refreshHistory = useCallback(async () => {
    try {
      setHistory(await api.history());
    } catch {
      /* history is non-critical; ignore */
    }
  }, []);

  // Initial load: health, saved profile, history.
  useEffect(() => {
    refreshHealth();
    refreshHistory();
    api
      .getProfile()
      .then(setProfile)
      .catch(() => {});

    // If we arrived here from the Email HR page, prefill the compose form and
    // remember the contact so we can mark it "sent" after the email goes out.
    const prefill = takeComposePrefill();
    if (prefill) {
      setCompose((c) => ({ ...c, ...prefill.input }));
      if (prefill.hrSent) setPendingHRSent(prefill.hrSent);
      setToast({
        kind: "info",
        message: `Composing to ${prefill.input.recipientName || prefill.input.recipientEmail}${
          prefill.input.company ? ` at ${prefill.input.company}` : ""
        }. Preview → Send when ready.`,
      });
    }
  }, [refreshHealth, refreshHistory]);

  // Reload the profile for the active track whenever it changes (each track has
  // its own saved profile). Skip the very first run — the mount effect already
  // loaded the SD profile.
  const firstTrackRun = useRef(true);
  useEffect(() => {
    if (firstTrackRun.current) {
      firstTrackRun.current = false;
      return;
    }
    setRendered(null);
    api
      .getProfile(track)
      .then(setProfile)
      .catch(() => {});
  }, [track]);

  const handleParse = async () => {
    setParsing(true);
    setToast(null);
    try {
      const p = await api.parseResume(track);
      setProfile(p);
      setToast({ kind: "info", message: `Parsed your ${track.toUpperCase()} resume. Review the fields, then save.` });
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setParsing(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setSaved(false);
    try {
      const p = await api.saveProfile(profile, track);
      setProfile(p);
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setSaving(false);
    }
  };

  const handlePreview = async () => {
    setPreviewing(true);
    setToast(null);
    try {
      // Ensure the latest profile edits are persisted before rendering.
      await api.saveProfile(profile, track);
      const r = await api.preview({ ...compose, track });
      setRendered(r);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setPreviewing(false);
    }
  };

  const handleSend = async () => {
    setSending(true);
    setToast(null);
    try {
      const res = await api.send({ ...compose, track });
      const how = res.source === "ai-tweaked" ? "AI-tailored" : "template";
      setToast({ kind: "success", message: `Sent ${how} email to ${res.sentTo} ✓` });
      setRendered(null);
      setCompose({ recipientEmail: "", recipientName: "", company: "", role: "" });
      // If this send originated from the Email HR page, move that contact to Sent.
      if (pendingHRSent) {
        api.hrMarkSent(pendingHRSent).catch(() => {});
        setPendingHRSent(null);
      }
      await Promise.all([refreshHistory(), refreshHealth()]);
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setSending(false);
    }
  };

  const handleDigest = async () => {
    setDigesting(true);
    setToast(null);
    try {
      const res = await api.sendDigest();
      setToast({
        kind: "success",
        message: `Digest of ${res.count} send${res.count === 1 ? "" : "s"} emailed to ${res.sentTo} ✓`,
      });
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setDigesting(false);
    }
  };

  // Merge a LinkedIn lookup result into the compose form. The email is the point
  // of the lookup so it always overwrites; name/company only fill when empty so
  // we never clobber something the user already typed.
  const handleLookupFound = (f: LookupFound) => {
    setCompose((c) => ({
      ...c,
      recipientEmail: f.email || c.recipientEmail,
      recipientName: c.recipientName?.trim() ? c.recipientName : f.name,
      company: c.company.trim() ? c.company : f.company,
    }));
  };

  return (
    <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
      {/* Top bar */}
      <header className="mb-6 flex flex-col gap-3 border-b border-[var(--border)] pb-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-bold tracking-tight sm:text-2xl">Resume Cold-Email Sender</h1>
            {health?.aiEnabled && (
              <span className="inline-flex items-center gap-1 rounded-full bg-[var(--accent)]/10 px-2.5 py-1 text-xs font-medium text-[var(--accent)]">
                ✨ AI · {health.aiModel}
              </span>
            )}
          </div>
          <p className="mt-1 text-sm text-[var(--muted)]">
            Type an email and company — send a tailored resume email via your Gmail.
          </p>
        </div>
        {/* Quick status pills — resume reflects the active track */}
        <div className="flex flex-wrap gap-2">
          <Pill
            ok={track === "ai" ? !!health?.hasResumeAI : !!health?.hasResumeSD}
            label={
              (track === "ai" ? health?.hasResumeAI : health?.hasResumeSD)
                ? `${track === "ai" ? "AI" : "SDE"} resume ready`
                : `No ${track === "ai" ? "AI" : "SDE"} resume`
            }
          />
          <Pill ok={!!health?.hasCredentials} label={health?.hasCredentials ? "Gmail connected" : "Gmail not set"} />
        </div>
      </header>

      {/* Alerts */}
      <div className="mb-5 space-y-3">
        <SetupBanner health={health} />
        {toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}
      </div>

      {/* Profile (collapsible setup, full width) */}
      <div className="mb-5">
        <ProfileEditor
          profile={profile}
          onChange={setProfile}
          onParse={handleParse}
          onSave={handleSave}
          parsing={parsing}
          saving={saving}
          saved={saved}
        />
      </div>

      {/* Dashboard grid: workflow (left) + status & activity (right) */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* Main workflow column */}
        <div className="space-y-5 lg:col-span-2">
          {health?.lookupEnabled && <LinkedInLookup onFound={handleLookupFound} />}
          <ComposeForm
            input={compose}
            onChange={setCompose}
            onPreview={handlePreview}
            loading={previewing}
            step={health?.lookupEnabled ? 2 : 1}
            track={track}
            onTrackChange={setTrack}
            hasResumeSD={!!health?.hasResumeSD}
            hasResumeAI={!!health?.hasResumeAI}
          />
          <BulkSend track={track} />
          {rendered ? (
            <EmailPreview
              rendered={rendered}
              recipient={compose.recipientEmail}
              gmailUser={health?.gmailUser ?? ""}
              onBack={() => setRendered(null)}
              onSend={handleSend}
              sending={sending}
            />
          ) : (
            <div className="rounded-xl border border-dashed border-[var(--border)] p-8 text-center text-sm text-[var(--muted)]">
              Fill in the recipient and company, then <span className="font-medium">Preview email</span> to see it here before sending.
            </div>
          )}
        </div>

        {/* Side column: status + activity feed */}
        <aside className="space-y-5">
          <StatusPanel health={health} history={history} />
          <SendHistory
            entries={history}
            digestEnabled={health?.digestEnabled ?? false}
            digestTo={health?.digestTo ?? ""}
            onDigest={handleDigest}
            digesting={digesting}
          />
        </aside>
      </div>

      <footer className="mt-8 border-t border-[var(--border)] pt-5 text-center text-xs text-[var(--muted)]">
        Sends through Gmail SMTP · Your resume and credentials stay on the backend.
      </footer>
    </main>
  );
}
