"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, Health, JobTimeRange, JobsState, api } from "@/lib/api";
import { JobSearch } from "@/components/JobSearch";
import { SectionHeader, Toast } from "@/components/ui";

type ToastState = { kind: "success" | "error" | "info"; message: string } | null;

/** Human-friendly "3h 20m" / "45m" from a seconds count. */
function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.ceil(seconds / 60)); // minutes, rounded up
  const h = Math.floor(total / 60);
  const m = total % 60;
  if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`;
  return `${m}m`;
}

export default function JobsPage() {
  const [health, setHealth] = useState<Health | null>(null);
  const [jobs, setJobs] = useState<JobsState>({ open: [], applied: [] });
  const [searching, setSearching] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const [limit, setLimit] = useState(50);
  const [timeRange, setTimeRange] = useState<JobTimeRange>("24h");

  const errMsg = (e: unknown) =>
    e instanceof ApiError ? e.message : "Something went wrong. Please try again.";

  const refreshJobs = useCallback(async () => {
    try {
      const j = await api.jobs();
      setJobs({
        open: j.open ?? [],
        applied: j.applied ?? [],
        blocked: j.blocked ?? false,
        retryAfter: j.retryAfter ?? 0,
        lastRunAt: j.lastRunAt,
      });
    } catch {
      /* jobs are non-critical on load; ignore */
    }
  }, []);

  useEffect(() => {
    api
      .health()
      .then(setHealth)
      .catch(() => {
        setHealth(null);
        setToast({
          kind: "error",
          message:
            "Cannot reach the backend at http://localhost:8080. Start it with: cd backend && go run .",
        });
      });
    refreshJobs();
  }, [refreshJobs]);

  const handleSearch = async () => {
    setSearching(true);
    setToast(null);
    try {
      const res = await api.searchJobs({ limit, timeRange });
      setJobs((j) => ({
        ...j,
        open: res.open ?? [],
        blocked: res.blocked,
        retryAfter: res.retryAfter,
        lastRunAt: res.lastRunAt ?? j.lastRunAt,
      }));
      if (res.blocked) {
        setToast({
          kind: "info",
          message: `A search already ran recently. To protect your Apify credits, the job search runs at most once every 6 hours — showing your saved jobs. Try again in ~${formatDuration(res.retryAfter)}.`,
        });
      } else if (res.added > 0) {
        setToast({
          kind: "success",
          message: `Added ${res.added} new job${res.added === 1 ? "" : "s"}.`,
        });
      } else {
        setToast({
          kind: "info",
          message: "No new jobs since your last search.",
        });
      }
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    } finally {
      setSearching(false);
    }
  };

  // Clicking Apply opens LinkedIn (via the anchor) and moves the job to Applied.
  const handleApply = async (id: string) => {
    try {
      const j = await api.markApplied(id);
      // Preserve the current rate-limit status (markApplied doesn't report it).
      setJobs((prev) => ({
        ...prev,
        open: j.open ?? [],
        applied: j.applied ?? [],
      }));
    } catch (e) {
      setToast({ kind: "error", message: errMsg(e) });
    }
  };

  return (
    <main className="mx-auto w-full max-w-4xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
      <SectionHeader
        title="Job Search"
        subtitle="Latest fresher (0–1 yr) software jobs in India, AI-screened for eligibility. Clicking Apply opens the job on LinkedIn and moves it to your Applied list."
      />

      {toast && (
        <div className="mb-5">
          <Toast kind={toast.kind} message={toast.message} onClose={() => setToast(null)} />
        </div>
      )}

      {health && !health.jobsEnabled ? (
        <div className="rounded-[var(--radius-lg)] border border-[var(--warning)]/40 bg-[var(--warning-soft)] px-4 py-3 text-sm text-[var(--warning-fg)]">
          Job search isn’t configured. Set <span className="font-mono">APIFY_TOKEN</span> in{" "}
          <span className="font-mono">backend/.env</span> and restart the backend.
        </div>
      ) : (
        <JobSearch
          jobs={jobs}
          onSearch={handleSearch}
          onApply={handleApply}
          loading={searching}
          blocked={jobs.blocked ?? false}
          retryLabel={jobs.retryAfter ? formatDuration(jobs.retryAfter) : ""}
          limit={limit}
          timeRange={timeRange}
          onLimitChange={setLimit}
          onTimeRangeChange={setTimeRange}
        />
      )}
    </main>
  );
}
