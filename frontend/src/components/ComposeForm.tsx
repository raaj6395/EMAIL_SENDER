"use client";

import { ComposeInput, Track } from "@/lib/api";
import { guessCompany, guessFirstName } from "@/lib/guess";
import { Button, Card, Field, Input, Segmented } from "./ui";

export const TRACK_OPTIONS: { value: Track; label: string }[] = [
  { value: "sd", label: "SDE" },
  { value: "ai", label: "AI" },
];

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

  // Auto-suggest company and first name from the email, only into empty fields.
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
            ? "AI resume and AI profile, tailored for AI and ML roles"
            : "SDE resume and SDE profile, tailored for software engineering roles"
        }
      >
        <div className="flex flex-wrap items-center gap-3">
          <Segmented value={track} options={TRACK_OPTIONS} onChange={onTrackChange} />
          {!resumeReady && (
            <span className="text-[12px] text-[var(--warning-fg)]">
              No {track === "ai" ? "AI" : "SDE"} resume yet. Drop one into Resumes
            </span>
          )}
        </div>
      </Field>

      <div className="mt-5 grid gap-4 sm:grid-cols-2">
        <Field label="Recipient email" hint="Company and name are suggested from the address">
          <Input
            type="email"
            value={input.recipientEmail}
            onChange={(e) => handleEmailChange(e.target.value)}
            placeholder="priya@carousell.com"
          />
        </Field>
        <Field label="Company">
          <Input value={input.company} onChange={(e) => onChange({ ...input, company: e.target.value })} placeholder="Carousell" />
        </Field>
        <Field label="Recipient name" hint="Optional. Used in the greeting">
          <Input
            value={input.recipientName ?? ""}
            onChange={(e) => onChange({ ...input, recipientName: e.target.value })}
            placeholder="Priya"
          />
        </Field>
        <Field label="Role" hint="Optional. Overrides your target role for this email">
          <Input
            value={input.role ?? ""}
            onChange={(e) => onChange({ ...input, role: e.target.value })}
            placeholder="Backend Engineer"
          />
        </Field>
      </div>

      <div className="mt-6 flex flex-wrap items-center gap-3">
        <Button onClick={onPreview} disabled={!canPreview} loading={loading}>
          Preview email
        </Button>
        {!canPreview && <span className="text-[12px] text-[var(--subtle)]">Needs a valid email and a company</span>}
      </div>
    </Card>
  );
}
