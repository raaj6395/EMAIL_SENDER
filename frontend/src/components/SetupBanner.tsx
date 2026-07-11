"use client";

import { Health } from "@/lib/api";
import { Toast } from "./ui";

export function SetupBanner({ health }: { health: Health | null }) {
  if (!health) return null;

  const missing: string[] = [];
  if (!health.hasCredentials) {
    missing.push(
      "Gmail credentials not configured. Add GMAIL_USER and GMAIL_APP_PASSWORD to backend/.env, then restart the backend."
    );
  }
  if (!health.hasResume) {
    missing.push("No resume found. Place your resume PDF at backend/data/resume.pdf.");
  }

  if (missing.length === 0) {
    return (
      <Toast
        kind="success"
        message={`Ready to send${health.gmailUser ? ` — sending as ${health.gmailUser}` : ""}. Resume and Gmail credentials detected.`}
      />
    );
  }

  return (
    <Toast
      kind="error"
      message={"Setup needed before you can send:\n• " + missing.join("\n• ")}
    />
  );
}
