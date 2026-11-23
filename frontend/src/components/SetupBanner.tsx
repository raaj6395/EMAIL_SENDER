"use client";

import { Health } from "@/lib/api";
import { Notice } from "./ui";

/** Shown only when something blocks sending. The header carries the ready state. */
export function SetupBanner({ health }: { health: Health | null }) {
  if (!health) return null;

  const missing: string[] = [];
  if (!health.hasCredentials) {
    missing.push("Add GMAIL_USER and GMAIL_APP_PASSWORD to backend/.env, then restart the backend");
  }
  if (!health.hasResume) {
    missing.push("Drop your SDE resume PDF into Resumes on the right");
  }
  if (missing.length === 0) return null;

  return (
    <Notice tone="danger">
      <p className="font-medium text-[var(--fg)]">Setup needed before you can send</p>
      <ul className="mt-1 list-disc space-y-0.5 pl-4">
        {missing.map((m) => (
          <li key={m}>{m}</li>
        ))}
      </ul>
    </Notice>
  );
}
