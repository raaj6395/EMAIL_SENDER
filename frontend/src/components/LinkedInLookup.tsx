"use client";

import { useState } from "react";
import { ApiError, LookupResult, api } from "@/lib/api";
import { Button, Card, Field, Input } from "./ui";

/** Found values passed up to auto-fill the compose form. */
export type LookupFound = { email: string; name: string; company: string };

// Status/confidence values that indicate a trustworthy email. Different actors
// use different vocab: MillionVerifier returns status "ok" + quality "good";
// others use "valid"/"deliverable" or a numeric score.
const GOOD_STATUS = new Set(["ok", "valid", "deliverable", "verified"]);
const GOOD_QUALITY = new Set(["good", "high", "excellent"]);

// A lookup email is "low confidence" if the actor flagged the status as anything
// other than a known-good value, or gave a numeric score below 80.
function isLowConfidence(r: LookupResult): boolean {
  const status = r.status.trim().toLowerCase();
  const conf = r.confidence.trim().toLowerCase();
  // If we have a recognized status, trust it.
  if (status) return !GOOD_STATUS.has(status);
  // Otherwise fall back to the quality/confidence field.
  if (conf) {
    const n = parseInt(conf, 10);
    if (!Number.isNaN(n)) return n < 80;
    return !GOOD_QUALITY.has(conf);
  }
  return false; // no signal at all — don't cry wolf
}

export function LinkedInLookup({ onFound }: { onFound: (f: LookupFound) => void }) {
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<LookupResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const urlValid = /linkedin\.com\/(in|sales)\//i.test(url.trim());

  const handleFind = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const r = await api.lookup(url.trim());
      setResult(r);
      if (r.found) {
        onFound({ email: r.email, name: r.name, company: r.company });
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Lookup failed. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card title="Find email from LinkedIn" step={1}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <Field label="LinkedIn profile URL" hint="Paste a profile link; we’ll look up their email and fill the form below.">
            <Input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && urlValid && !loading) handleFind();
              }}
              placeholder="https://www.linkedin.com/in/username/"
              disabled={loading}
            />
          </Field>
        </div>
        <Button onClick={handleFind} disabled={!urlValid} loading={loading} className="sm:mb-[2px]">
          {loading ? "Searching…" : "Find email"}
        </Button>
      </div>

      {loading && (
        <div className="mt-3 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--muted)]">
          Looking up the email — this can take <strong>1–2 minutes</strong> (the actor
          scrapes the profile and verifies the address). Hang tight…
        </div>
      )}

      {error && (
        <div className="mt-3 rounded-[var(--radius-md)] border border-[var(--danger)]/40 bg-[var(--danger-soft)] px-3 py-2 text-sm text-[var(--danger-fg)]">
          {error}
        </div>
      )}

      {result && !result.found && !error && (
        <div className="mt-3 rounded-[var(--radius-md)] border border-[var(--warning)]/40 bg-[var(--warning-soft)] px-3 py-2 text-sm text-[var(--warning-fg)]">
          No email found for this profile. Enter the recipient email manually below.
        </div>
      )}

      {result && result.found && (
        <div className="mt-3 rounded-[var(--radius-md)] border border-[var(--success)]/30 bg-[var(--success-soft)] px-3 py-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium text-[var(--success-fg)]">Found:</span>
            <span className="font-mono">{result.email}</span>
            {result.confidence && (
              <span className="rounded-[var(--radius-full)] bg-[var(--elevated)] px-2 py-0.5 text-xs text-[var(--muted)]">
                confidence {result.confidence}
              </span>
            )}
            {result.status && (
              <span className="rounded-[var(--radius-full)] bg-[var(--elevated)] px-2 py-0.5 text-xs text-[var(--muted)]">
                {result.status}
              </span>
            )}
          </div>
          <div className="mt-1 text-xs text-[var(--muted)]">
            {[result.name, result.company].filter(Boolean).join(" · ") || "Filled into the compose form below."}
          </div>
          {isLowConfidence(result) && (
            <div className="mt-2 text-xs font-medium text-[var(--warning-fg)]">
              ⚠ This email looks low-confidence — double-check it before sending.
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
