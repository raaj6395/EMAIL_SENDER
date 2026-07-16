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

/** Which resume/profile track an email uses. */
export type Track = "sd" | "ai";

export interface Health {
  ok: boolean;
  hasResume: boolean;
  hasResumeSD: boolean;
  hasResumeAI: boolean;
  hasCredentials: boolean;
  gmailUser: string;
  aiEnabled: boolean;
  aiModel: string;
  digestEnabled: boolean;
  digestTo: string;
  lookupEnabled: boolean;
  jobsEnabled: boolean;
  hrEnabled: boolean;
}

export interface ComposeInput {
  recipientEmail: string;
  recipientName?: string;
  company: string;
  role?: string;
  track?: Track; // "sd" (default) or "ai" — selects resume/profile/AI flavor
}

/** One recipient in a bulk send. */
export interface BatchItem {
  email: string;
  company: string;
  name: string;
  status: "queued" | "sending" | "sent" | "failed" | "skipped";
  error?: string;
}

/** Live status of the bulk-send queue. */
export interface BatchStatus {
  active: boolean;
  track: Track;
  total: number;
  sent: number;
  failed: number;
  remaining: number;
  nextInSec: number;
  items: BatchItem[];
  startedAt?: string;
  done: boolean;
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

/** Time-range values the job-search actor supports (excluding the too-narrow "1h"). */
export type JobTimeRange = "24h" | "7d" | "6m";

/** Dropdown options for the time-range selector. */
export const JOB_TIME_RANGES: { value: JobTimeRange; label: string }[] = [
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last week" },
  { value: "6m", label: "Last 6 months" },
];

/** Dropdown options for the job-count selector (10…100 by tens). */
export const JOB_LIMITS: number[] = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100];

/** An HR/recruiter contact from the uploaded spreadsheet. */
export interface HRContact {
  company: string;
  name: string;
  role: string;
  email?: string;
  phone?: string; // display form, e.g. "+91 63954 86191"
  waPhone?: string; // digits-only for wa.me, e.g. "916395486191"
  rank: number; // company importance (0-100), higher = more important
}

/** A contact already reached out to (in the Sent section). */
export interface HRSentRecord {
  key: string;
  channel: "email" | "whatsapp";
  company: string;
  name: string;
  role: string;
  email?: string;
  phone?: string;
  sentAt: string;
}

/** WhatsApp send-rate status (guards against getting the number flagged). */
export interface HRRateStatus {
  sentInWindow: number; // sends within the rolling window
  windowCap: number; // max sends allowed per window
  windowHours: number; // window length in hours
  cooldownLeft: number; // seconds until the inter-send cooldown clears
  resetIn: number; // seconds until the cap frees up (oldest send ages out)
  blocked: boolean; // true if a send is not allowed right now
  capReached: boolean; // true if the window cap is hit
}

/** One page of HR contacts, plus the full sent list for that channel. */
export interface HRPage {
  contacts: HRContact[];
  total: number;
  page: number;
  pageSize: number;
  sent: HRSentRecord[];
  rate?: HRRateStatus; // present for the WhatsApp channel
}

/** Payload to mark a contact as reached out to. */
export interface MarkSentInput {
  channel: "email" | "whatsapp";
  company: string;
  name: string;
  role?: string;
  email?: string;
  phone?: string;
  key?: string;
}

/** Build the query string for an HR list request. */
function hrQuery(opts?: { page?: number; pageSize?: number; q?: string }): string {
  const p = new URLSearchParams();
  if (opts?.page) p.set("page", String(opts.page));
  if (opts?.pageSize) p.set("pageSize", String(opts.pageSize));
  if (opts?.q?.trim()) p.set("q", opts.q.trim());
  return p.toString();
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
  parseResume: (track: Track = "sd") =>
    request<Profile>(`/api/parse-resume?track=${track}`, { method: "POST" }),
  getProfile: (track: Track = "sd") => request<Profile>(`/api/profile?track=${track}`),
  saveProfile: (p: Profile, track: Track = "sd") =>
    request<Profile>(`/api/profile?track=${track}`, { method: "PUT", body: JSON.stringify(p) }),
  preview: (input: ComposeInput) =>
    request<ComposeResult>("/api/preview", { method: "POST", body: JSON.stringify(input) }),
  send: (input: ComposeInput) =>
    request<SendResult>("/api/send", { method: "POST", body: JSON.stringify(input) }),
  batchStart: (rows: string, track: Track) =>
    request<BatchStatus>("/api/batch", {
      method: "POST",
      body: JSON.stringify({ rows, track }),
    }),
  batchStatus: () => request<BatchStatus>("/api/batch"),
  batchCancel: () => request<BatchStatus>("/api/batch/cancel", { method: "POST" }),
  history: () => request<HistoryEntry[]>("/api/history"),
  sendDigest: () => request<DigestResult>("/api/digest", { method: "POST" }),
  lookup: (linkedinUrl: string) =>
    request<LookupResult>("/api/lookup", {
      method: "POST",
      body: JSON.stringify({ linkedinUrl }),
    }),
  searchJobs: (opts?: { limit?: number; timeRange?: JobTimeRange; roles?: string[] }) =>
    request<SearchResult>("/api/jobs/search", {
      method: "POST",
      body: JSON.stringify({
        roles: opts?.roles,
        limit: opts?.limit,
        timeRange: opts?.timeRange,
      }),
    }),
  hrWhatsApp: (opts?: { page?: number; pageSize?: number; q?: string }) =>
    request<HRPage>(
      `/api/hr/whatsapp?${hrQuery(opts)}`
    ),
  hrEmail: (opts?: { page?: number; pageSize?: number; q?: string }) =>
    request<HRPage>(`/api/hr/email?${hrQuery(opts)}`),
  hrRerank: () => request<{ ok: boolean; companies: number }>("/api/hr/rerank", { method: "POST" }),
  hrMarkSent: (input: MarkSentInput) =>
    request<{ sent: HRSentRecord[]; rate?: HRRateStatus }>("/api/hr/sent", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  jobs: () => request<JobsState>("/api/jobs"),
  markApplied: (id: string) =>
    request<JobsState>("/api/jobs/applied", {
      method: "POST",
      body: JSON.stringify({ id }),
    }),
};

// ---- Compose prefill handoff (Email HR page → main compose page) ----
// sessionStorage carries a chosen contact across the client-side navigation to
// "/", where the compose form is populated from it (then it's cleared).

const PREFILL_KEY = "composePrefill";

/** A compose prefill, optionally tagged so the main page can mark the source HR
 *  contact as "sent" after the email actually goes out. */
export interface ComposePrefill {
  input: ComposeInput;
  hrSent?: MarkSentInput; // present when coming from the Email HR page
}

export function setComposePrefill(v: ComposePrefill): void {
  try {
    sessionStorage.setItem(PREFILL_KEY, JSON.stringify(v));
  } catch {
    /* storage unavailable — ignore */
  }
}

/** Reads and clears any pending compose prefill. Returns null if none. */
export function takeComposePrefill(): ComposePrefill | null {
  try {
    const raw = sessionStorage.getItem(PREFILL_KEY);
    if (!raw) return null;
    sessionStorage.removeItem(PREFILL_KEY);
    return JSON.parse(raw) as ComposePrefill;
  } catch {
    return null;
  }
}
