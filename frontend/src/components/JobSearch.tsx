"use client";

import { JobsState, StoredJob } from "@/lib/api";
import { Button, Card } from "./ui";

function formatWhen(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Small rounded pill for a job attribute (experience, arrangement, etc.). */
function Pill({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded-full bg-[var(--background)] px-2 py-0.5 text-xs text-[var(--muted)]">
      {children}
    </span>
  );
}

/** Green/amber verdict badge. "not" jobs are filtered out server-side. */
function VerdictBadge({ verdict }: { verdict: StoredJob["verdict"] }) {
  const eligible = verdict === "eligible";
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
        eligible
          ? "bg-green-500/15 text-green-700 dark:text-green-300"
          : "bg-amber-500/15 text-amber-700 dark:text-amber-300"
      }`}
    >
      {eligible ? "Eligible" : "Maybe"}
    </span>
  );
}

/** One open job card, with an Apply link that also moves it to Applied. */
function OpenJobRow({
  job,
  onApply,
}: {
  job: StoredJob;
  onApply: (id: string) => void;
}) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate font-medium">{job.title}</div>
          <div className="mt-0.5 truncate text-xs text-[var(--muted)]">
            {[job.organization, job.location].filter(Boolean).join(" · ")}
          </div>
        </div>
        <a
          href={job.url}
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => onApply(job.id)}
          className="shrink-0 rounded-lg bg-[var(--accent)] px-3 py-1.5 text-sm font-medium text-white transition hover:opacity-90"
        >
          Apply ↗
        </a>
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        <VerdictBadge verdict={job.verdict} />
        {job.experienceLevel && <Pill>{job.experienceLevel} yrs</Pill>}
        {job.workArrangement && <Pill>{job.workArrangement}</Pill>}
        {job.employmentType && <Pill>{job.employmentType.replace(/_/g, " ")}</Pill>}
      </div>

      {job.reason && (
        <div className="mt-2 text-xs text-[var(--muted)]">
          <span className="font-medium">AI:</span> {job.reason}
        </div>
      )}
    </div>
  );
}

/** One applied job card (muted, with the applied timestamp). */
function AppliedJobRow({ job }: { job: StoredJob }) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2.5 text-sm">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="inline-block h-2 w-2 shrink-0 rounded-full bg-green-500" />
          <a
            href={job.url}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate font-medium hover:underline"
          >
            {job.title}
          </a>
        </div>
        <div className="mt-0.5 truncate pl-4 text-xs text-[var(--muted)]">
          {[job.organization, job.location].filter(Boolean).join(" · ")}
        </div>
      </div>
      {job.appliedAt && (
        <span className="shrink-0 whitespace-nowrap text-xs text-[var(--muted)]">
          {formatWhen(job.appliedAt)}
        </span>
      )}
    </div>
  );
}

export function JobSearch({
  jobs,
  onSearch,
  onApply,
  loading,
  blocked = false,
  retryLabel = "",
}: {
  jobs: JobsState;
  onSearch: () => void;
  onApply: (id: string) => void;
  loading: boolean;
  blocked?: boolean; // true when a fresh Apify run is rate-limited right now
  retryLabel?: string; // e.g. "3h 20m" until the next run is allowed
}) {
  // Default to empty arrays so a null/absent list from the backend can never
  // crash the render (e.g. an older backend that returns null instead of []).
  const open = jobs.open ?? [];
  const applied = jobs.applied ?? [];

  return (
    <Card
      title="Job search — fresher software roles in India"
      action={
        <Button
          variant="primary"
          onClick={() => onSearch()}
          loading={loading}
          disabled={blocked}
          title={
            blocked
              ? `Runs at most once every 6 hours to protect Apify credits${
                  retryLabel ? ` — try again in ~${retryLabel}` : ""
                }`
              : undefined
          }
        >
          {loading ? "Finding…" : "Find jobs"}
        </Button>
      }
    >
      {blocked && (
        <div className="mb-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
          To protect your Apify credits, the job search runs at most{" "}
          <strong>once every 6 hours</strong>.
          {retryLabel && <> Next search available in ~{retryLabel}.</>} Showing your
          saved jobs below.
        </div>
      )}

      {loading && (
        <div className="mb-3 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm text-[var(--muted)]">
          Fetching the latest jobs and checking each one for eligibility — this can
          take <strong>up to a minute</strong>. Hang tight…
        </div>
      )}

      {/* Open jobs */}
      <div className="mb-2 text-sm font-medium">
        Open jobs{open.length > 0 && <span className="text-[var(--muted)]"> · {open.length}</span>}
      </div>
      {open.length === 0 ? (
        <div className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center text-sm text-[var(--muted)]">
          No open jobs yet — click <span className="font-medium">Find jobs</span> to
          fetch eligible roles.
        </div>
      ) : (
        <div className="space-y-3">
          {open.map((job) => (
            <OpenJobRow key={job.id} job={job} onApply={onApply} />
          ))}
        </div>
      )}

      {/* Applied jobs */}
      <div className="mb-2 mt-6 text-sm font-medium">
        Applied{applied.length > 0 && <span className="text-[var(--muted)]"> · {applied.length}</span>}
      </div>
      {applied.length === 0 ? (
        <div className="text-sm text-[var(--muted)]">
          Nothing applied yet. Clicking a job’s <span className="font-medium">Apply</span>{" "}
          link opens it on LinkedIn and moves it here.
        </div>
      ) : (
        <div className="space-y-2">
          {applied.map((job) => (
            <AppliedJobRow key={job.id} job={job} />
          ))}
        </div>
      )}
    </Card>
  );
}
