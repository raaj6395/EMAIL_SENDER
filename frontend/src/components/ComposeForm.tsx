"use client";

import { ComposeInput } from "@/lib/api";
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

  return (
    <Card title="Compose" step={2}>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Recipient email">
          <Input
            type="email"
            value={input.recipientEmail}
            onChange={(e) => onChange({ ...input, recipientEmail: e.target.value })}
            placeholder="hiring@company.com"
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
