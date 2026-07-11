// Typed client for the Go email-sender backend.

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface Profile {
  name: string;
  email: string;
  phone: string;
  targetRole: string;
  skills: string[];
  pitch: string;
  linkedin: string;
  github: string;
  portfolio: string;
}

export interface Health {
  ok: boolean;
  hasResume: boolean;
  hasCredentials: boolean;
  gmailUser: string;
  aiEnabled: boolean;
  aiModel: string;
  digestEnabled: boolean;
  digestTo: string;
}

export interface ComposeInput {
  recipientEmail: string;
  recipientName?: string;
  company: string;
  role?: string;
}

export interface Rendered {
  subject: string;
  bodyHTML: string;
  bodyText: string;
  attachmentName: string;
}

/** Preview result: rendered email plus how it was produced. */
export interface ComposeResult extends Rendered {
  source: "ai" | "template";
  note?: string;
}

export interface SendResult {
  ok: boolean;
  subject: string;
  sentTo: string;
  source: "ai" | "template";
}

export interface DigestResult {
  ok: boolean;
  sentTo: string;
  count: number;
}

export interface HistoryEntry {
  recipientEmail: string;
  company: string;
  subject: string;
  status: "sent" | "failed";
  error?: string;
  sentAt: string;
}

export const emptyProfile = (): Profile => ({
  name: "",
  email: "",
  phone: "",
  targetRole: "",
  skills: [],
  pitch: "",
  linkedin: "",
  github: "",
  portfolio: "",
});

/** ApiError carries the backend's human-readable message. */
export class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${BASE}${path}`, {
      ...init,
      headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    });
  } catch {
    throw new ApiError(
      `Cannot reach the backend at ${BASE}. Is it running? (cd backend && go run .)`
    );
  }

  const text = await res.text();
  const data = text ? JSON.parse(text) : {};
  if (!res.ok) {
    throw new ApiError(data?.error || `Request failed (${res.status})`);
  }
  return data as T;
}

export const api = {
  health: () => request<Health>("/api/health"),
  parseResume: () => request<Profile>("/api/parse-resume", { method: "POST" }),
  getProfile: () => request<Profile>("/api/profile"),
  saveProfile: (p: Profile) =>
    request<Profile>("/api/profile", { method: "PUT", body: JSON.stringify(p) }),
  preview: (input: ComposeInput) =>
    request<ComposeResult>("/api/preview", { method: "POST", body: JSON.stringify(input) }),
  send: (input: ComposeInput) =>
    request<SendResult>("/api/send", { method: "POST", body: JSON.stringify(input) }),
  history: () => request<HistoryEntry[]>("/api/history"),
  sendDigest: () => request<DigestResult>("/api/digest", { method: "POST" }),
};
