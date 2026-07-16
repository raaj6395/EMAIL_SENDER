"use client";

import { ComposeInput, Track } from "@/lib/api";
import { guessCompany, guessFirstName } from "@/lib/guess";
import { Button, Card, Field, Input } from "./ui";

export function ComposeForm({
  input,
  onChange,
  onPreview,
  loading,
  step = 1,
  track,
  onTrackChange,
  hasResumeSD = false,
  hasResumeAI = false,
}: {
  input: ComposeInput;
  onChange: (i: ComposeInput) => void;
  onPreview: () => void;
  loading: boolean;
  step?: number;
  track: Track;
  onTrackChange: (t: Track) => void;
  hasResumeSD?: boolean;
  hasResumeAI?: boolean;
}) {
  const emailValid = /\S+@\S+\.\S+/.test(input.recipientEmail.trim());
  const companyValid = input.company.trim().length > 0;
  const canPreview = emailValid && companyValid;

  // When the email changes, auto-suggest company + first name — but only into
  // fields the user hasn't already filled, so we never overwrite their input.
  const handleEmailChange = (email: string) => {
    const next: ComposeInput = { ...input, recipientEmail: email };
    if (!input.company.trim()) {
      const company = guessCompany(email);
      if (company) next.company = company;
    }
    if (!(input.recipientName ?? "").trim()) {
      const name = guessFirstName(email);
      if (name) next.recipientName = name;
    }
    onChange(next);
  };

  const resumeReady = track === "ai" ? hasResumeAI : hasResumeSD;

  return (
    <Card title="Compose" step={step}>
      <Field
        label="Profile"
        hint={
          track === "ai"
            ? "Uses your AI resume + AI profile, tailored for AI/ML roles."
            : "Uses your SDE resume + SDE profile, tailored for software-engineering roles."
        }
      >
        <div className="flex flex-wrap items-center gap-3">
          <div className="inline-flex overflow-hidden rounded-lg border border-[var(--border)]">
            {(["sd", "ai"] as const).map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => onTrackChange(t)}
                className={`px-4 py-1.5 text-sm font-medium transition ${
                  track === t
                    ? "bg-[var(--accent)] text-white"
                    : "bg-[var(--background)] text-[var(--muted)] hover:text-[var(--foreground)]"
                }`}
              >
                {t === "sd" ? "SDE" : "AI"}
              </button>
            ))}
          </div>
          {!resumeReady && (
            <span className="text-xs text-amber-600 dark:text-amber-400">
              ⚠ No {track === "ai" ? "ai_resume.pdf" : "resume.pdf"} found for this profile.
            </span>
          )}
        </div>
      </Field>

      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        <Field label="Recipient email" hint="Company & name are auto-suggested — edit if needed">
          <Input
            type="email"
            value={input.recipientEmail}
            onChange={(e) => handleEmailChange(e.target.value)}
            placeholder="priya@carousell.com"
          />
        </Field>
        <Field label="Company name">
          <Input
            value={input.company}
            onChange={(e) => onChange({ ...input, company: e.target.value })}
            placeholder="Carousell"
          />
        </Field>
      </div>
      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        <Field label="Recipient name (optional)" hint="Used in the greeting: “Hi {name},”">
          <Input
            value={input.recipientName ?? ""}
            onChange={(e) => onChange({ ...input, recipientName: e.target.value })}
            placeholder="Priya"
          />
        </Field>
        <Field label="Role (optional)" hint="Overrides your target role for this email only">
          <Input
            value={input.role ?? ""}
            onChange={(e) => onChange({ ...input, role: e.target.value })}
            placeholder="Senior Backend Engineer"
          />
        </Field>
      </div>
      <div className="mt-5 flex items-center gap-3">
        <Button onClick={onPreview} disabled={!canPreview} loading={loading}>
          Preview email
        </Button>
        {!canPreview && (
          <span className="text-xs text-[var(--muted)]">
            Enter a valid email and company to preview.
          </span>
        )}
      </div>
    </Card>
  );
}
