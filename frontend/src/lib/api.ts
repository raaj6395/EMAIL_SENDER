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
  reachEnabled: boolean;
  inboxEnabled: boolean;
  scheduleEnabled: boolean;
}

/** A triaged inbox reply to your outreach. */
export interface InboxReply {
  messageId: string;
  fromEmail: string;
  fromName: string;
  subject: string;
  body: string;
  receivedAt: string;
  category: "positive" | "question" | "follow_up_later" | "not_open" | "other";
  draft: string;
  reason: string;
  status: "needs_reply" | "scheduled" | "no_action" | "replied" | "dismissed";
  followUpAt?: string;
}

export interface ComposeInput {
  recipientEmail: string;
  recipientName?: string;
  company: string;
  role?: string;
  track?: Track; // "sd" (default) or "ai" — selects resume/profile/AI flavor
}

/** One NDJSON line from the streaming existence check. */
export interface ExistsLine {
  email?: string;
  company?: string;
  name?: string;
  kind?: "exist" | "invalid" | "unknown";
  queued?: boolean;
  done?: boolean;
  error?: string;
  checked?: number;
  exist?: number;
  invalid?: number;
  unknown?: number;
  added?: number;
}

/** Result of the pre-send email check (syntax + MX). */
export interface VerifyResult {
  email: string;
  valid: boolean; // false only for clear failures (bad syntax / no MX)
  code: "ok" | "bad_syntax" | "no_mx" | "error";
  reason: string;
}

/** One recipient in the scheduled send queue. */
export interface BatchItem {
  email: string;
  company: string;
  name: string;
  status: "queued" | "sending" | "sent" | "failed" | "skipped";
  error?: string;
  sentAt?: string;
}

/** Live status of the scheduled send queue. */
export interface BatchStatus {
  track: Track;
  total: number;
  sent: number;
  failed: number;
  remaining: number;
  paused: boolean;
  /** Window override active — draining regardless of the schedule. */
  sendNow: boolean;
  items: BatchItem[];

  /** A send window is open right now. */
  windowOpen: boolean;
  /** Actively draining: window open, work left, not paused, cap not hit. */
  sending: boolean;
  nextInSec: number;
  /** RFC3339 start of the next window (when closed, or when capped). */
  nextWindowAt: string;
  opensInSec: number;
  windowEndsAt: string;
  sentInWindow: number;
  perWindowCap: number; // 0 = uncapped
  capReached: boolean;
  queuedAt?: string;
  lastSentAt?: string;
}

/** Result of adding recipients to the queue. */
export interface QueueAddResult {
  added: number;
  skipped: number; // duplicates already in the queue
  queue: BatchStatus;
}

/** One weekly send window. `day` is 0=Sunday … 6=Saturday; times are minutes
 *  from midnight in the server's local timezone. */
export interface SendWindow {
  day: number;
  startMin: number;
  endMin: number;
  label?: string;
}

/** The weekly send schedule plus derived "what happens next" state. */
export interface SendSchedule {
  enabled: boolean;
  windows: SendWindow[];
  perWindowCap: number;
  open: boolean;
  nextWindowAt?: string;
  opensInSec: number;
  windowEndsAt?: string;
  timezone: string;
}

export const WEEKDAY_NAMES = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
];

/** "11:00" ⇄ minutes-from-midnight helpers for the schedule editor. */
export function minToHHMM(min: number): string {
  const h = Math.floor(min / 60);
  const m = min % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}

export function hhmmToMin(s: string): number {
  const [h, m] = s.split(":").map((n) => parseInt(n, 10));
  if (Number.isNaN(h) || Number.isNaN(m)) return 0;
  return h * 60 + m;
}

/** Human "in 2d 3h" style countdown from seconds. */
export function formatCountdown(sec: number): string {
  if (sec <= 0) return "now";
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  return `${sec}s`;
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

// ---- Target companies (who to cold-email, by fresher comp band) ----

/** The kind of employer — shapes both comp and how you pitch. */
export type TargetCategory = "hypergrowth" | "mnc" | "gcc" | "product" | "fintech" | "other";

/** One target employer with an AI estimate of its fresher SDE comp in India. */
export interface TargetCompany {
  name: string;
  minLPA: number;
  maxLPA: number;
  category: TargetCategory;
  confidence: "high" | "medium" | "low";
  hq: string;
  note: string;
  careersUrl: string;
  emailDomain: string;
  tech: string[];
  addedAt: string;
  source: "ai" | "manual";
}

export interface TargetsPage {
  companies: TargetCompany[];
  total: number;
  cached: number;
  aiEnabled: boolean;
}

export const TARGET_CATEGORIES: { value: TargetCategory; label: string }[] = [
  { value: "hypergrowth", label: "Hypergrowth startup" },
  { value: "mnc", label: "MNC / Big Tech" },
  { value: "gcc", label: "GCC / R&D centre" },
  { value: "product", label: "Indian product co." },
  { value: "fintech", label: "Fintech" },
];

export function targetCategoryLabel(c: TargetCategory): string {
  return TARGET_CATEGORIES.find((x) => x.value === c)?.label ?? "Other";
}

/** Comp floors offered in the Targets filter. */
export const LPA_FLOORS = [12, 15, 20, 25, 30, 40];

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
  channel: "email" | "whatsapp" | "reach";
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
  channel: "email" | "whatsapp" | "reach";
  company: string;
  name: string;
  role?: string;
  email?: string;
  phone?: string;
  key?: string;
}

/** A deliverability check result cached for one address. */
export interface ReachVerdict {
  email: string;
  valid: boolean;
  code: "ok" | "bad_syntax" | "no_mx" | "error";
  reason: string;
  checkedAt: string;
}

/** The AI's judgement of one company against the fresher-SDE / 12+ LPA bar. */
export interface ReachScreen {
  company: string;
  hiresSDE: boolean;
  fresher: boolean;
  minLPA: number;
  maxLPA: number;
  confidence: "high" | "medium" | "low";
  note: string;
  screenedAt: string;
}

/** One shortlisted contact from the outreach sheet. */
export interface ReachContact {
  company: string;
  name: string;
  emails: string[];
  location: string;
  website: string;
  industry: string;
  email: string; // the best deliverable address
  verified?: ReachVerdict;
  screen?: ReachScreen;
}

/** How much of the sheet the pipeline has processed so far. */
export interface ReachProgress {
  rows: number;
  companies: number;
  verified: number;
  pendingVerify: number;
  screened: number;
  pendingScreen: number;
  shortlisted: number;
}

/** One page of the reach shortlist. */
export interface ReachPage {
  contacts: ReachContact[];
  total: number;
  page: number;
  pageSize: number;
  progress: ReachProgress;
  sent: HRSentRecord[];
  floorLPA: number;
}

/** Build the query string for a reach list request. */
function reachQuery(opts?: {
  page?: number;
  pageSize?: number;
  q?: string;
  minLPA?: number;
  all?: boolean;
}): string {
  const p = new URLSearchParams();
  if (opts?.page) p.set("page", String(opts.page));
  if (opts?.pageSize) p.set("pageSize", String(opts.pageSize));
  if (opts?.q?.trim()) p.set("q", opts.q.trim());
  if (opts?.minLPA) p.set("minLPA", String(opts.minLPA));
  if (opts?.all) p.set("all", "1");
  return p.toString();
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
      // FormData sets its own multipart boundary header.
      headers:
        init?.body instanceof FormData
          ? init.headers
          : { "Content-Type": "application/json", ...(init?.headers ?? {}) },
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
  /** Replace a track's resume PDF. Every later email on that track attaches it. */
  uploadResume: (file: File, track: Track) => {
    const form = new FormData();
    form.append("file", file);
    return request<{ ok: boolean; track: Track; bytes: number }>(`/api/resume/upload?track=${track}`, {
      method: "POST",
      body: form,
    });
  },
  /** URL of the resume PDF currently on file for a track. */
  resumeUrl: (track: Track) => `${BASE}/api/resume?track=${track}`,
  getProfile: (track: Track = "sd") => request<Profile>(`/api/profile?track=${track}`),
  saveProfile: (p: Profile, track: Track = "sd") =>
    request<Profile>(`/api/profile?track=${track}`, { method: "PUT", body: JSON.stringify(p) }),
  preview: (input: ComposeInput) =>
    request<ComposeResult>("/api/preview", { method: "POST", body: JSON.stringify(input) }),
  send: (input: ComposeInput) =>
    request<SendResult>("/api/send", { method: "POST", body: JSON.stringify(input) }),
  /** Add recipients to the scheduled queue — they go out in the next window. */
  queueAdd: (rows: string, track: Track) =>
    request<QueueAddResult>("/api/batch", {
      method: "POST",
      body: JSON.stringify({ rows, track }),
    }),
  verifyEmail: (email: string) =>
    request<VerifyResult>("/api/verify-email", { method: "POST", body: JSON.stringify({ email }) }),
  /** Stream a bulk existence check. Each confirmed address is queued as it is found. */
  checkExists: async (rows: string, track: Track, onLine: (line: ExistsLine) => void, signal?: AbortSignal) => {
    let res: Response;
    try {
      res = await fetch(`${BASE}/api/exists`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rows, track }),
        signal,
      });
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      throw new ApiError(
        `Cannot reach the backend at ${BASE}. Is it running? (cd backend && go run .)`
      );
    }
    if (!res.ok) {
      const text = await res.text();
      let message = `Request failed (${res.status})`;
      try {
        const data = text ? JSON.parse(text) : {};
        if (data?.error) message = data.error;
      } catch {
        /* body was not JSON */
      }
      throw new ApiError(message);
    }
    if (!res.body) throw new ApiError("The check returned an empty response.");

    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      let nl = buf.indexOf("\n");
      while (nl >= 0) {
        const line = buf.slice(0, nl).trim();
        buf = buf.slice(nl + 1);
        if (line) onLine(JSON.parse(line) as ExistsLine);
        nl = buf.indexOf("\n");
      }
    }
    const rest = buf.trim();
    if (rest) onLine(JSON.parse(rest) as ExistsLine);
  },
  queueStatus: () => request<BatchStatus>("/api/batch"),
  queuePause: () => request<BatchStatus>("/api/batch/pause", { method: "POST" }),
  queueResume: () => request<BatchStatus>("/api/batch/resume", { method: "POST" }),
  /** Drop everything still waiting (keeps the sent/failed log). */
  queueClearWaiting: () => request<BatchStatus>("/api/batch/cancel", { method: "POST" }),
  /** Empty the queue entirely, including finished items. */
  queueClearAll: () => request<BatchStatus>("/api/batch/clear", { method: "POST" }),
  /** Send everything waiting right now, ignoring the schedule and window cap. */
  queueSendNow: () =>
    request<{ sending: number; queue: BatchStatus }>("/api/batch/send-now", { method: "POST" }),
  /** Return to the normal schedule. */
  queueSendNowCancel: () =>
    request<BatchStatus>("/api/batch/send-now/cancel", { method: "POST" }),
  /** Re-queue failed recipients (they were never delivered, so no duplicates). */
  queueRetryFailed: () =>
    request<{ requeued: number; queue: BatchStatus }>("/api/batch/retry-failed", { method: "POST" }),
  schedule: () => request<SendSchedule>("/api/schedule"),
  saveSchedule: (s: { enabled: boolean; perWindowCap: number; windows: SendWindow[] }) =>
    request<SendSchedule>("/api/schedule", {
      method: "PUT",
      body: JSON.stringify({
        enabled: s.enabled,
        perWindowCap: s.perWindowCap,
        windows: s.windows.map((w) => ({ day: w.day, startMin: w.startMin, endMin: w.endMin })),
      }),
    }),
  history: () => request<HistoryEntry[]>("/api/history"),
  sendDigest: () => request<DigestResult>("/api/digest", { method: "POST" }),
  lookup: (linkedinUrl: string) =>
    request<LookupResult>("/api/lookup", {
      method: "POST",
      body: JSON.stringify({ linkedinUrl }),
    }),
  searchJobs: (opts?: { limit?: number; timeRange?: JobTimeRange; roles?: string[]; force?: boolean }) =>
    request<SearchResult>("/api/jobs/search", {
      method: "POST",
      body: JSON.stringify({
        roles: opts?.roles,
        limit: opts?.limit,
        timeRange: opts?.timeRange,
        force: opts?.force ?? false,
      }),
    }),
  targets: (opts?: { minLPA?: number; categories?: TargetCategory[]; q?: string }) => {
    const p = new URLSearchParams();
    if (opts?.minLPA) p.set("minLPA", String(opts.minLPA));
    if (opts?.categories?.length) p.set("categories", opts.categories.join(","));
    if (opts?.q?.trim()) p.set("q", opts.q.trim());
    return request<TargetsPage>(`/api/targets?${p.toString()}`);
  },
  /** Ask the AI for more companies matching the profile (excludes ones already found). */
  targetsDiscover: (opts: {
    minLPA?: number;
    categories?: TargetCategory[];
    cities?: string[];
    count?: number;
  }) =>
    request<{ added: TargetCompany[]; count: number; cached: number }>("/api/targets/discover", {
      method: "POST",
      body: JSON.stringify(opts),
    }),
  targetsAdd: (name: string) =>
    request<TargetCompany>("/api/targets/add", { method: "POST", body: JSON.stringify({ name }) }),
  targetsRemove: (name: string) =>
    request<{ ok: boolean }>("/api/targets/remove", {
      method: "POST",
      body: JSON.stringify({ name }),
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
  repliesCheck: (limit: number) =>
    request<{ replies: InboxReply[]; checked: number; matched: number }>("/api/replies/check", {
      method: "POST",
      body: JSON.stringify({ limit }),
    }),
  replies: () => request<{ replies: InboxReply[] }>("/api/replies"),
  replySend: (messageId: string, body: string) =>
    request<{ ok: boolean; replies: InboxReply[] }>("/api/replies/send", {
      method: "POST",
      body: JSON.stringify({ messageId, body }),
    }),
  replyDismiss: (messageId: string) =>
    request<{ ok: boolean; replies: InboxReply[] }>("/api/replies/dismiss", {
      method: "POST",
      body: JSON.stringify({ messageId }),
    }),
  /** The shortlist: sheet contacts joined with their verification + screening. */
  reach: (opts?: { page?: number; pageSize?: number; q?: string; minLPA?: number; all?: boolean }) =>
    request<ReachPage>(`/api/reach?${reachQuery(opts)}`),
  /** Verify the next chunk of addresses (syntax + MX). */
  reachVerify: (limit?: number) =>
    request<{ checked: number; valid: number; bad: number }>("/api/reach/verify", {
      method: "POST",
      body: JSON.stringify({ limit }),
    }),
  /** AI-screen the next chunk of companies against the fresher / 12+ LPA bar. */
  reachScreen: (limit?: number) =>
    request<{ screened: number; qualified: number }>("/api/reach/screen", {
      method: "POST",
      body: JSON.stringify({ limit }),
    }),
  /** Queue contacts for the next send window. */
  reachQueue: (contacts: { email: string; company: string; name: string }[], track: Track) =>
    request<QueueAddResult>("/api/reach/queue", {
      method: "POST",
      body: JSON.stringify({ contacts, track }),
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
