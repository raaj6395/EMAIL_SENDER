"use client";

import { ComposeInput } from "@/lib/api";
import { guessCompany, guessFirstName } from "@/lib/guess";
import { Button, Card, Field, Input } from "./ui";

export function ComposeForm({
  input,
  onChange,
  onPreview,
  loading,
}: {
  input: ComposeInput;
  onChange: (i: ComposeInput) => void;
  onPreview: () => void;
  loading: boolean;
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

  return (
    <Card title="Compose" step={1}>
      <div className="grid gap-4 sm:grid-cols-2">
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
