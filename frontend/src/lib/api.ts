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
  lookupEnabled: boolean;
  jobsEnabled: boolean;
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
  source: "ai-tweaked" | "template";
  note?: string;
}

export interface SendResult {
  ok: boolean;
  subject: string;
  sentTo: string;
  source: "ai-tweaked" | "template";
}

export interface DigestResult {
  ok: boolean;
  sentTo: string;
  count: number;
}

/** Result of a LinkedIn URL → email lookup. */
export interface LookupResult {
  found: boolean;
  email: string;
  name: string;
  company: string;
  confidence: string; // e.g. "92" or ""
  status: string; // e.g. "valid" / "risky" / "unknown"
}

export interface HistoryEntry {
  recipientEmail: string;
  company: string;
  subject: string;
  status: "sent" | "failed";
  error?: string;
  sentAt: string;
}

/** A job returned by the LinkedIn job search. */
export interface Job {
  id: string;
  title: string;
  organization: string;
  organizationUrl: string;
  organizationLogo: string;
  url: string; // LinkedIn apply link
  location: string;
  seniority: string;
  experienceLevel: string; // e.g. "0-2"
  employmentType: string; // e.g. "FULL_TIME"
  workArrangement: string; // e.g. "On-site" / "Remote"
  datePosted: string;
  description: string;
  requirementsSummary: string;
  keySkills: string[];
}

/** A job plus its AI eligibility verdict and (once applied) the applied time. */
export interface StoredJob extends Job {
  verdict: "eligible" | "maybe" | "not";
  reason: string;
  appliedAt?: string;
}

/** The persisted open + applied job lists (plus rate-limit status on load). */
export interface JobsState {
  open: StoredJob[];
  applied: StoredJob[];
  blocked?: boolean; // true when a fresh Apify run is rate-limited right now
  retryAfter?: number; // seconds until a new run is allowed
  lastRunAt?: string; // ISO time of the most recent Apify run
}

/** Result of a job search. `blocked` = the actor was NOT called (rate-limited). */
export interface SearchResult {
  open: StoredJob[];
  added: number; // count of newly-added jobs
  blocked: boolean; // true when a real run happened within the rate-limit window
  retryAfter: number; // seconds until a new run is allowed
  lastRunAt?: string; // ISO time of the most recent Apify run (when blocked)
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
  lookup: (linkedinUrl: string) =>
    request<LookupResult>("/api/lookup", {
      method: "POST",
      body: JSON.stringify({ linkedinUrl }),
    }),
  searchJobs: (roles?: string[]) =>
    request<SearchResult>("/api/jobs/search", {
      method: "POST",
      body: JSON.stringify({ roles }),
    }),
  jobs: () => request<JobsState>("/api/jobs"),
  markApplied: (id: string) =>
    request<JobsState>("/api/jobs/applied", {
      method: "POST",
      body: JSON.stringify({ id }),
    }),
};
