"use client";

import { useRef, useState } from "react";
import { ApiError, Track, api } from "@/lib/api";
import { Card, Spinner, buttonClass } from "./ui";
import { IconExternal, IconUpload } from "./icons";

const MAX_BYTES = 10 << 20; // matches the backend's upload limit

function formatSize(bytes: number) {
  return bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** A resume drawn as a sheet of paper. Dashed outline when nothing is on file. */
function Sheet({ present }: { present: boolean }) {
  if (!present) {
    return (
      <div className="flex h-[60px] w-[46px] shrink-0 items-center justify-center rounded-[4px] border border-dashed border-[var(--border-strong)] text-[var(--subtle)]">
        <IconUpload />
      </div>
    );
  }
  return (
    <div
      className="relative h-[60px] w-[46px] shrink-0 bg-[var(--paper)] shadow-[0_1px_2px_rgba(31,27,22,0.12),0_4px_10px_rgba(31,27,22,0.08)] ring-1 ring-black/5"
      style={{ clipPath: "polygon(0 0, calc(100% - 12px) 0, 100% 12px, 100% 100%, 0 100%)", borderRadius: 3 }}
      aria-hidden="true"
    >
      <span className="absolute right-0 top-0 h-3 w-3 bg-[var(--paper-line)]" style={{ clipPath: "polygon(0 0, 0 100%, 100% 100%)" }} />
      <span className="absolute left-[7px] top-[9px] h-[3px] w-[18px] rounded-full bg-[var(--paper-ink)]/70" />
      <span className="absolute left-[7px] top-[17px] h-[2px] w-[30px] rounded-full bg-[var(--paper-line)]" />
      <span className="absolute left-[7px] top-[22px] h-[2px] w-[26px] rounded-full bg-[var(--paper-line)]" />
      <span className="absolute left-[7px] top-[27px] h-[2px] w-[30px] rounded-full bg-[var(--paper-line)]" />
      <span className="absolute left-[7px] top-[32px] h-[2px] w-[20px] rounded-full bg-[var(--paper-line)]" />
      <span className="absolute bottom-[6px] left-[7px] font-mono text-[8px] font-medium leading-none text-[var(--accent)]">PDF</span>
    </div>
  );
}

/** One drop slot for one profile's resume. */
function Slot({
  track,
  present,
  selected,
  onUploaded,
}: {
  track: Track;
  present: boolean;
  selected: boolean;
  onUploaded: () => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [last, setLast] = useState<{ name: string; size: number } | null>(null);
  const label = track === "ai" ? "AI resume" : "SDE resume";

  const upload = async (file?: File) => {
    if (!file) return;
    setError(null);
    if (file.type !== "application/pdf" && !file.name.toLowerCase().endsWith(".pdf")) {
      setError(`${file.name} is not a PDF`);
      return;
    }
    if (file.size > MAX_BYTES) {
      setError(`${file.name} is ${formatSize(file.size)}, the limit is 10 MB`);
      return;
    }
    setBusy(true);
    try {
      const res = await api.uploadResume(file, track);
      setLast({ name: file.name, size: res.bytes });
      onUploaded();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Upload failed");
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  };

  const meta = busy
    ? "Uploading"
    : last
      ? `${last.name} · ${formatSize(last.size)} · uploaded now`
      : present
        ? `Attached to every ${track === "ai" ? "AI" : "SDE"} email`
        : "Drop a PDF here or browse";

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={(e) => {
        // Ignore leaves into child elements, or the highlight flickers.
        if (!e.currentTarget.contains(e.relatedTarget as Node)) setOver(false);
      }}
      onDrop={(e) => {
        e.preventDefault();
        setOver(false);
        upload(e.dataTransfer.files?.[0]);
      }}
      className={`rounded-[var(--radius-md)] border p-3 transition-colors duration-150 ${
        over
          ? "border-dashed border-[var(--accent)] bg-[var(--accent-soft)]"
          : selected
            ? "border-[var(--border-strong)] bg-[var(--surface)]"
            : "border-[var(--border)]"
      }`}
    >
      <div className="flex items-center gap-3">
        <Sheet present={present} />
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <span className="text-[14px] font-medium text-[var(--fg)]">{label}</span>
            {selected && <span className="text-[12px] text-[var(--subtle)]">In use</span>}
          </div>
          <div className={`mt-0.5 truncate text-[12px] ${present || busy ? "text-[var(--muted)]" : "text-[var(--warning-fg)]"}`}>
            {over ? `Drop to replace the ${label}` : meta}
          </div>
          <div className="mt-2 flex items-center gap-2">
            <button type="button" onClick={() => input.current?.click()} disabled={busy} className={buttonClass("secondary", "sm")}>
              {busy ? <Spinner /> : <IconUpload />}
              {present ? "Replace" : "Upload"}
            </button>
            {present && (
              <a href={api.resumeUrl(track)} target="_blank" rel="noopener noreferrer" className={buttonClass("ghost", "sm")}>
                View
                <IconExternal size={14} />
              </a>
            )}
          </div>
        </div>
      </div>
      {error && <p className="mt-2 text-[12px] text-[var(--danger-fg)]">{error}</p>}
      <input
        ref={input}
        type="file"
        accept="application/pdf,.pdf"
        className="hidden"
        onChange={(e) => upload(e.target.files?.[0])}
      />
    </div>
  );
}

/** Both profiles' resumes. Drop a PDF on a slot to replace what gets attached. */
export function ResumeUpload({
  track,
  hasResumeSD,
  hasResumeAI,
  onUploaded,
}: {
  track: Track;
  hasResumeSD: boolean;
  hasResumeAI: boolean;
  onUploaded: () => void;
}) {
  return (
    <Card title="Resumes">
      <div className="space-y-2">
        <Slot track="sd" present={hasResumeSD} selected={track === "sd"} onUploaded={onUploaded} />
        <Slot track="ai" present={hasResumeAI} selected={track === "ai"} onUploaded={onUploaded} />
      </div>
      <p className="mt-3 text-[12px] leading-4 text-[var(--subtle)]">
        PDF up to 10 MB. The new file goes out with the next email you send
      </p>
    </Card>
  );
}
