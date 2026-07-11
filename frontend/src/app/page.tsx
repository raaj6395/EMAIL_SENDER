"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  ComposeInput,
  ComposeResult,
  Health,
  HistoryEntry,
  Profile,
  api,
  emptyProfile,
} from "@/lib/api";
import { SetupBanner } from "@/components/SetupBanner";
import { ProfileEditor } from "@/components/ProfileEditor";
import { ComposeForm } from "@/components/ComposeForm";
import { EmailPreview } from "@/components/EmailPreview";
import { SendHistory } from "@/components/SendHistory";
import { Toast } from "@/components/ui";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

export default function Home() {
  const [health, setHealth] = useState<Health | null>(null);
  const [profile, setProfile] = useState<Profile>(emptyProfile());
  const [compose, setCompose] = useState<ComposeInput>({ recipientEmail: "", recipientName: "", company: "", role: "" });
  const [rendered, setRendered] = useState<ComposeResult | null>(null);
  const [history, setHistory] = useState<HistoryEntry[]>([]);

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
  }, [refreshHealth, refreshHistory]);

  const handleParse = async () => {
    setParsing(true);
    setToast(null);
    try {
      const p = await api.parseResume();
      setProfile(p);
      setToast({ kind: "info", message: "Parsed your resume. Review the fields, then save." });
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
      const p = await api.saveProfile(profile);
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
      await api.saveProfile(profile);
      const r = await api.preview(compose);
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
      const res = await api.send(compose);
      const how = res.source === "ai" ? "AI-written" : "template";
      setToast({ kind: "success", message: `Sent ${how} email to ${res.sentTo} ✓` });
      setRendered(null);
      setCompose({ recipientEmail: "", recipientName: "", company: "", role: "" });
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

  return (
    <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-8 sm:py-12">
      <header className="mb-6">
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-2xl font-bold tracking-tight">Resume Cold-Email Sender</h1>
          {health?.aiEnabled && (
            <span className="inline-flex items-center gap-1 rounded-full bg-[var(--accent)]/10 px-2.5 py-1 text-xs font-medium text-[var(--accent)]">
              ✨ AI on{health.aiModel ? ` · ${health.aiModel}` : ""}
            </span>
          )}
        </div>
        <p className="mt-1 text-sm text-[var(--muted)]">
          Enter an email and company — send a tailored resume email via your Gmail.
        </p>
      </header>

      <div className="space-y-5">
        <SetupBanner health={health} />
        {toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}

        <ProfileEditor
          profile={profile}
          onChange={setProfile}
          onParse={handleParse}
          onSave={handleSave}
          parsing={parsing}
          saving={saving}
          saved={saved}
        />

        <ComposeForm
          input={compose}
          onChange={setCompose}
          onPreview={handlePreview}
          loading={previewing}
        />

        {rendered && (
          <EmailPreview
            rendered={rendered}
            recipient={compose.recipientEmail}
            gmailUser={health?.gmailUser ?? ""}
            onBack={() => setRendered(null)}
            onSend={handleSend}
            sending={sending}
          />
        )}

        <SendHistory
          entries={history}
          digestEnabled={health?.digestEnabled ?? false}
          digestTo={health?.digestTo ?? ""}
          onDigest={handleDigest}
          digesting={digesting}
        />
      </div>

      <footer className="mt-10 text-center text-xs text-[var(--muted)]">
        Sends through Gmail SMTP · Your resume and credentials stay on the backend.
      </footer>
    </main>
  );
}
