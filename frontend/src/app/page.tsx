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
import { SendScheduleCard } from "@/components/SendSchedule";
import { EmailPreview } from "@/components/EmailPreview";
import { SendHistory } from "@/components/SendHistory";
import { StatusPanel } from "@/components/StatusPanel";
import { ResumeUpload } from "@/components/ResumeUpload";
import { IconMail } from "@/components/icons";
import { Alerts, EmptyState, Page, SectionHeader, StatusPill, Toast } from "@/components/ui";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

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
          "Cannot reach the backend at http://localhost:8080. Start it with cd backend && go run .",
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
        }. Preview, then send when ready`,
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
      setToast({ kind: "info", message: `Filled from your ${track === "ai" ? "AI" : "SDE"} resume. Check the fields, then save` });
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
      // Pre-send check: block a clearly-undeliverable recipient (bad syntax /
      // dead domain) before we bother rendering or sending.
      const check = await api.verifyEmail(compose.recipientEmail);
      if (!check.valid) {
        setToast({ kind: "error", message: check.reason });
        return;
      }
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
      setToast({ kind: "success", message: `Sent the ${how} email to ${res.sentTo}` });
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
        message: `Digest of ${res.count} send${res.count === 1 ? "" : "s"} emailed to ${res.sentTo}`,
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

  const resumeOn = track === "ai" ? !!health?.hasResumeAI : !!health?.hasResumeSD;

  return (
    <Page wide>
      <SectionHeader
        title="Email sender"
        subtitle="A tailored application email to one company, sent from your Gmail with the right resume attached"
        action={
          health && (
            <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
              <StatusPill ok={health.hasCredentials} label={health.hasCredentials ? health.gmailUser || "Gmail connected" : "Gmail not set"} />
              <StatusPill ok={resumeOn} label={`${track === "ai" ? "AI" : "SDE"} resume ${resumeOn ? "on file" : "missing"}`} />
              {health.aiEnabled && (
                <span className="font-mono text-[12px] text-[var(--subtle)]">AI · {health.aiModel}</span>
              )}
            </div>
          )
        }
      />

      <Alerts>
        <SetupBanner health={health} />
        {toast && <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />}
      </Alerts>

      <div className="mb-6">
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

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="min-w-0 space-y-6">
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
          {rendered ? (
            <EmailPreview
              rendered={rendered}
              recipient={compose.recipientEmail}
              gmailUser={health?.gmailUser ?? ""}
              onBack={() => setRendered(null)}
              onSend={handleSend}
              sending={sending}
              step={health?.lookupEnabled ? 3 : 2}
              track={track}
            />
          ) : (
            <EmptyState icon={<IconMail size={20} />} title="The preview shows up here">
              Add a recipient and company, then choose Preview email to read the exact message before it goes out
            </EmptyState>
          )}
          <BulkSend track={track} />
          <SendScheduleCard />
        </div>

        <aside className="min-w-0 space-y-6">
          <ResumeUpload
            track={track}
            hasResumeSD={!!health?.hasResumeSD}
            hasResumeAI={!!health?.hasResumeAI}
            onUploaded={refreshHealth}
          />
          <StatusPanel history={history} />
          <SendHistory
            entries={history}
            digestEnabled={health?.digestEnabled ?? false}
            digestTo={health?.digestTo ?? ""}
            onDigest={handleDigest}
            digesting={digesting}
          />
        </aside>
      </div>

      <footer className="mt-12 border-t border-[var(--border)] pt-6 text-[12px] text-[var(--subtle)]">
        Sends through Gmail SMTP. Your resumes and credentials stay on the backend
      </footer>
    </Page>
  );
}
