"use client";

import { useState } from "react";
import { ApiError, LookupResult, api } from "@/lib/api";
import { Button, Card, Field, Input, Notice } from "./ui";

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
    <Card title="Find an email from LinkedIn" step={1}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <Field label="LinkedIn profile URL" hint="Paste a profile link to look up the email and fill the form below">
            <Input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && urlValid && !loading) handleFind();
              }}
              placeholder="linkedin.com/in/username"
              disabled={loading}
            />
          </Field>
        </div>
        <Button onClick={handleFind} disabled={!urlValid} loading={loading} className="sm:mb-6">
          {loading ? "Searching" : "Find email"}
        </Button>
      </div>

      {loading && (
        <Notice className="mt-4">
          Looking up the email. This takes 1 to 2 minutes while the profile is read and the address verified
        </Notice>
      )}

      {error && (
        <Notice tone="danger" className="mt-4">
          {error}
        </Notice>
      )}

      {result && !result.found && !error && (
        <Notice tone="warning" className="mt-4">
          No email found for this profile. Type the recipient email below
        </Notice>
      )}

      {result && result.found && (
        <Notice tone={isLowConfidence(result) ? "warning" : "success"} className="mt-4">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="font-mono text-[13px] text-[var(--fg)]">{result.email}</span>
            {result.confidence && <span className="font-mono text-[12px] text-[var(--subtle)]">confidence {result.confidence}</span>}
            {result.status && <span className="font-mono text-[12px] text-[var(--subtle)]">{result.status}</span>}
          </div>
          <div className="mt-0.5">
            {[result.name, result.company].filter(Boolean).join(" · ") || "Filled into the compose form below"}
          </div>
          {isLowConfidence(result) && (
            <div className="mt-1 text-[var(--warning-fg)]">Low confidence. Double-check this address before sending</div>
          )}
        </Notice>
      )}
    </Card>
  );
}
