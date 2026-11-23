"use client";

import {
  JOB_LIMITS,
  JOB_TIME_RANGES,
  JobTimeRange,
  JobsState,
  StoredJob,
} from "@/lib/api";
import { Badge, Button, Card, EmptyRow, Notice, Postmark, Select, SubHeading, buttonClass } from "./ui";
import { ContactRow, RowList } from "./hr";
import { IconExternal } from "./icons";

/** Quiet attribute tag (experience, arrangement, type). */
function Tag({ children }: { children: React.ReactNode }) {
  return <span className="text-[12px] text-[var(--muted)]">{children}</span>;
}

/** One open job, with an Apply link that also moves it to Applied. */
function OpenJobRow({ job, onApply }: { job: StoredJob; onApply: (id: string) => void }) {
  const eligible = job.verdict === "eligible";
  const tags = [
    job.experienceLevel && `${job.experienceLevel} yrs`,
    job.workArrangement,
    job.employmentType?.replace(/_/g, " "),
  ].filter(Boolean) as string[];
  return (
    <ContactRow
      actions={
        <a
          href={job.url}
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => onApply(job.id)}
          className={buttonClass("primary", "sm")}
        >
          Apply
          <IconExternal size={14} />
        </a>
      }
    >
      <div className="truncate text-[14px] font-medium">{job.title}</div>
      <div className="mt-0.5 truncate text-[13px] text-[var(--muted)]">
        {[job.organization, job.location].filter(Boolean).join(" · ")}
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
        <Badge tone={eligible ? "success" : "warning"}>{eligible ? "Eligible" : "Maybe"}</Badge>
        {tags.map((t) => (
          <Tag key={t}>{t}</Tag>
        ))}
      </div>
      {job.reason && <p className="mt-2 text-[13px] leading-5 text-[var(--muted)]">{job.reason}</p>}
    </ContactRow>
  );
}

/** One applied job, with its postmark. */
function AppliedJobRow({ job }: { job: StoredJob }) {
  return (
    <div className="flex items-center justify-between gap-4 px-4 py-3">
      <div className="min-w-0">
        <a href={job.url} target="_blank" rel="noopener noreferrer" className="block truncate text-[14px] font-medium hover:underline">
          {job.title}
        </a>
        <div className="mt-0.5 truncate text-[13px] text-[var(--muted)]">
          {[job.organization, job.location].filter(Boolean).join(" · ")}
        </div>
      </div>
      {job.appliedAt && <Postmark iso={job.appliedAt} sent />}
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
  limit,
  timeRange,
  onLimitChange,
  onTimeRangeChange,
}: {
  jobs: JobsState;
  onSearch: (force?: boolean) => void;
  onApply: (id: string) => void;
  loading: boolean;
  blocked?: boolean; // true when a fresh Apify run is rate-limited right now
  retryLabel?: string; // e.g. "3h 20m" until the next run is allowed
  limit: number;
  timeRange: JobTimeRange;
  onLimitChange: (n: number) => void;
  onTimeRangeChange: (t: JobTimeRange) => void;
}) {
  // Default to empty arrays so a null/absent list from the backend can never
  // crash the render (e.g. an older backend that returns null instead of []).
  const open = jobs.open ?? [];
  const applied = jobs.applied ?? [];

  return (
    <Card
      title="Fresher software roles"
      action={
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-2 text-[13px] text-[var(--muted)]">
            <span className="hidden sm:inline">Posted</span>
            <Select
              className="w-auto"
              value={timeRange}
              onChange={(e) => onTimeRangeChange(e.target.value as JobTimeRange)}
              disabled={loading}
            >
              {JOB_TIME_RANGES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </Select>
          </label>
          <label className="flex items-center gap-2 text-[13px] text-[var(--muted)]">
            <span className="hidden sm:inline">Show</span>
            <Select
              className="w-auto"
              value={limit}
              onChange={(e) => onLimitChange(Number(e.target.value))}
              disabled={loading}
            >
              {JOB_LIMITS.map((n) => (
                <option key={n} value={n}>
                  {n} jobs
                </option>
              ))}
            </Select>
          </label>
          <Button
            variant="primary"
            onClick={() => onSearch(false)}
            loading={loading && !blocked}
            disabled={blocked}
            title={
              blocked
                ? `Runs at most once every 6 hours to protect Apify credits${
                    retryLabel ? `. Try again in about ${retryLabel}` : ""
                  }`
                : undefined
            }
          >
            {loading && !blocked ? "Finding" : "Find jobs"}
          </Button>
          {blocked && (
            <Button
              variant="secondary"
              onClick={() => onSearch(true)}
              loading={loading}
              title="Ignore the 6-hour limit and run a fresh search now (spends Apify credit and resets the timer)"
            >
              {loading ? "Searching" : "Force search"}
            </Button>
          )}
        </div>
      }
    >
      {blocked && (
        <Notice tone="warning" className="mb-4">
          Search runs at most once every 6 hours to protect your Apify credits
          {retryLabel && <>. Next search in about {retryLabel}</>}. Showing saved jobs, or use Force search to run now
        </Notice>
      )}

      {loading && (
        <Notice className="mb-4">Fetching the latest jobs and checking each for eligibility. This takes up to a minute</Notice>
      )}

      {/* Open jobs */}
      <SubHeading count={open.length}>Open</SubHeading>
      {open.length === 0 ? (
        <EmptyRow>No open jobs yet. Find jobs fetches eligible roles</EmptyRow>
      ) : (
        <RowList>
          {open.map((job) => (
            <OpenJobRow key={job.id} job={job} onApply={onApply} />
          ))}
        </RowList>
      )}

      {/* Applied jobs */}
      <div className="mt-8">
        <SubHeading count={applied.length}>Applied</SubHeading>
        {applied.length === 0 ? (
          <p className="text-[13px] text-[var(--muted)]">Nothing applied yet. Apply opens the job on LinkedIn and moves it here</p>
        ) : (
          <RowList>
            {applied.map((job) => (
              <AppliedJobRow key={job.id} job={job} />
            ))}
          </RowList>
        )}
      </div>
    </Card>
  );
}
