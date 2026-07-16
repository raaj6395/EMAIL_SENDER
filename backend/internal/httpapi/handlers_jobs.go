package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"emailsender/internal/jobs"
	"emailsender/internal/resume"
)

// jobRunWindow is the minimum time between actual (paid) Apify actor runs. If a
// run happened within this window, a new "Find jobs" request is served entirely
// from the saved open list and the actor is NOT called — a hard cap of at most
// one paid run per window. This is not overridable by the client.
const jobRunWindow = 6 * time.Hour

// handleJobSearch fetches fresher India software jobs from Apify, runs an AI
// eligibility check on each, drops the clearly-ineligible ones, merges the rest
// into the persisted open list, and returns the updated open list. This is the
// slow call (Apify run + one AI call per job), so the UI shows a spinner.
//
// Rate limit: if a real Apify run is recorded within jobRunWindow, the actor is
// NOT called — the request returns the cached open list with blocked=true. Every
// actual run is appended to the runs log (jobs_runs.json).
func (s *Server) handleJobSearch(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasJobs() {
		writeError(w, http.StatusBadRequest, "job search not configured: set APIFY_TOKEN in backend/.env")
		return
	}
	var in struct {
		Roles     []string `json:"roles"`
		Limit     int      `json:"limit"`
		TimeRange string   `json:"timeRange"`
	}
	// Body is optional; ignore decode errors so an empty body uses defaults.
	_ = json.NewDecoder(r.Body).Decode(&in)

	// Hard rate-limit: if a real run happened within the window, do NOT call the
	// actor. Return the saved open list with blocked=true. Not overridable.
	if last, ok := jobs.LastRunTime(s.cfg.JobsRunsPath); ok {
		if retryAfter := jobRunWindow - time.Since(last); retryAfter > 0 {
			open, err := jobs.LoadOpen(s.cfg.JobsOpenPath, s.cfg.JobsAppliedPath)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not read jobs: "+err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"open":       open,
				"added":      0,
				"blocked":    true,
				"retryAfter": int(retryAfter.Seconds()),
				"lastRunAt":  last,
			})
			return
		}
	}

	found, err := jobs.Search(r.Context(), s.jobsConfig(), in.Roles, jobs.DefaultLocation, in.Limit, in.TimeRange)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Load the saved profile so the AI can judge fit against the candidate.
	profile, _ := resume.LoadProfile(s.cfg.ProfilePath)
	if profile == nil {
		profile = &resume.Profile{}
	}

	// Run the eligibility checks concurrently (N is small — up to 25). Each
	// failure falls back to "maybe" so a job is never dropped due to an AI error.
	scored := make([]jobs.StoredJob, len(found))
	var wg sync.WaitGroup
	for i := range found {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			j := found[i]
			verdict := jobs.VerdictMaybe
			reason := ""
			if s.cfg.HasAI() {
				if e, err := jobs.CheckEligibility(r.Context(), s.cfg.OpenAIKey, s.cfg.OpenAIModel, profile, j); err == nil {
					verdict = e.Verdict
					reason = e.Reason
				}
			}
			scored[i] = jobs.StoredJob{Job: j, Verdict: verdict, Reason: reason}
		}(i)
	}
	wg.Wait()

	// Keep eligible + maybe; drop clearly-ineligible jobs.
	keep := make([]jobs.StoredJob, 0, len(scored))
	for _, j := range scored {
		if j.Verdict == jobs.VerdictNot {
			continue
		}
		keep = append(keep, j)
	}

	open, added, err := jobs.MergeOpen(s.cfg.JobsOpenPath, s.cfg.JobsAppliedPath, keep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save jobs: "+err.Error())
		return
	}

	// Record this actual (paid) run so the rate-limit window starts now.
	if err := jobs.AppendRun(s.cfg.JobsRunsPath, jobs.RunRecord{
		RanAt:     time.Now(),
		JobsFound: len(found),
		JobsAdded: added,
	}); err != nil {
		log.Printf("warning: could not record Apify run: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"open":       open,
		"added":      added,
		"blocked":    false,
		"retryAfter": int(jobRunWindow.Seconds()),
	})
}

// handleJobsList returns the persisted open and applied job lists.
func (s *Server) handleJobsList(w http.ResponseWriter, r *http.Request) {
	open, err := jobs.LoadOpen(s.cfg.JobsOpenPath, s.cfg.JobsAppliedPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read open jobs: "+err.Error())
		return
	}
	applied, err := jobs.LoadApplied(s.cfg.JobsAppliedPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read applied jobs: "+err.Error())
		return
	}

	// Tell the client if searching is currently rate-limited, so it can show the
	// state on load without spending a request to discover it.
	resp := map[string]any{
		"open":       open,
		"applied":    applied,
		"blocked":    false,
		"retryAfter": 0,
	}
	if last, ok := jobs.LastRunTime(s.cfg.JobsRunsPath); ok {
		if retryAfter := jobRunWindow - time.Since(last); retryAfter > 0 {
			resp["blocked"] = true
			resp["retryAfter"] = int(retryAfter.Seconds())
			resp["lastRunAt"] = last
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleMarkApplied moves a job from the open list to the applied list.
func (s *Server) handleMarkApplied(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(in.ID) == "" {
		writeError(w, http.StatusBadRequest, "a job id is required")
		return
	}
	_, applied, err := jobs.MarkApplied(s.cfg.JobsOpenPath, s.cfg.JobsAppliedPath, in.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update jobs: "+err.Error())
		return
	}
	// Return the FILTERED open list (blocked companies, duplicates, and applied
	// jobs removed) — the same view every other endpoint returns, so the UI never
	// shows raw leftovers after an apply.
	open, err := jobs.LoadOpen(s.cfg.JobsOpenPath, s.cfg.JobsAppliedPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read jobs: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"open":    open,
		"applied": applied,
	})
}
