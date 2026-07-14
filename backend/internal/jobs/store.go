package jobs

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// StoredJob is a Job plus its AI eligibility verdict and (once applied) the
// time it was moved to the Applied list.
type StoredJob struct {
	Job
	Verdict   string     `json:"verdict"` // "eligible" | "maybe" | "not"
	Reason    string     `json:"reason"`
	AppliedAt *time.Time `json:"appliedAt,omitempty"`
}

// storeMu guards concurrent read/modify/write of the jobs files. A single mutex
// covers both files because MarkApplied touches them together.
var storeMu sync.Mutex

func loadJobsLocked(path string) ([]StoredJob, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []StoredJob{}, nil
		}
		return nil, err
	}
	var jobs []StoredJob
	if err := json.Unmarshal(b, &jobs); err != nil {
		return nil, err
	}
	// A file containing literal "null" (e.g. written by an older build) unmarshals
	// to a nil slice, which would re-serialize as null. Normalize to an empty
	// slice so every response is a JSON array.
	if jobs == nil {
		jobs = []StoredJob{}
	}
	return jobs, nil
}

func saveJobsLocked(path string, jobs []StoredJob) error {
	b, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadOpen reads the open-jobs list for display, hiding anything that shouldn't
// appear in Open: blocked-company listings, content-duplicate postings (the
// actor returns the same job with different IDs), and any job already in the
// applied list (so an applied job never re-surfaces in Open, even if it lingered
// in the file from an older run). Missing file → empty slice.
func LoadOpen(openPath, appliedPath string) ([]StoredJob, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	jobs, err := loadJobsLocked(openPath)
	if err != nil {
		return nil, err
	}
	applied, err := loadJobsLocked(appliedPath)
	if err != nil {
		return nil, err
	}
	// Seed the seen-set with applied jobs so applied postings are hidden from Open.
	seen := make(map[string]bool, len(applied)+len(jobs))
	for _, j := range applied {
		seen[j.DedupKey()] = true
	}
	filtered := jobs[:0]
	for _, j := range jobs {
		key := j.DedupKey()
		if isBlockedCompany(j.Organization) || seen[key] {
			continue // blocked, already-applied, or a duplicate we've kept once
		}
		seen[key] = true
		filtered = append(filtered, j)
	}
	return filtered, nil
}

// LoadApplied reads the applied-jobs list. Missing file → empty slice.
func LoadApplied(appliedPath string) ([]StoredJob, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return loadJobsLocked(appliedPath)
}

// MergeOpen adds freshly-fetched jobs to the open list, de-duped by content
// (Job.DedupKey — the actor returns the same posting with different IDs, so ID
// dedup isn't enough) and excluding any job already present in the applied list
// (so a job you've applied to never reappears). Newly-added jobs are prepended
// (newest first). It returns the resulting open list (never nil) and the count
// of newly-added jobs.
func MergeOpen(openPath, appliedPath string, incoming []StoredJob) (open []StoredJob, added int, err error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	open, err = loadJobsLocked(openPath)
	if err != nil {
		return nil, 0, err
	}
	applied, err := loadJobsLocked(appliedPath)
	if err != nil {
		return nil, 0, err
	}

	seen := make(map[string]bool, len(open)+len(applied))
	for _, j := range open {
		seen[j.DedupKey()] = true
	}
	for _, j := range applied {
		seen[j.DedupKey()] = true
	}

	fresh := make([]StoredJob, 0, len(incoming))
	for _, j := range incoming {
		key := j.DedupKey()
		if j.ID == "" || seen[key] {
			continue
		}
		seen[key] = true
		fresh = append(fresh, j)
	}

	open = append(fresh, open...)
	// Guarantee a non-nil slice so the JSON response is [] rather than null.
	if open == nil {
		open = []StoredJob{}
	}
	if err := saveJobsLocked(openPath, open); err != nil {
		return nil, 0, err
	}
	return open, len(fresh), nil
}

// MarkApplied moves the job with the given ID from the open list to the applied
// list, stamping AppliedAt. It is a no-op (no error) if the ID isn't in the open
// list. Returns the updated open and applied lists.
func MarkApplied(openPath, appliedPath, jobID string) (open, applied []StoredJob, err error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	open, err = loadJobsLocked(openPath)
	if err != nil {
		return nil, nil, err
	}
	applied, err = loadJobsLocked(appliedPath)
	if err != nil {
		return nil, nil, err
	}

	var moved *StoredJob
	remaining := make([]StoredJob, 0, len(open))
	for i := range open {
		if open[i].ID == jobID && moved == nil {
			j := open[i]
			moved = &j
			continue
		}
		remaining = append(remaining, open[i])
	}

	// Not in open — nothing to do (already applied or unknown id).
	if moved == nil {
		return open, applied, nil
	}

	now := time.Now()
	moved.AppliedAt = &now
	applied = append([]StoredJob{*moved}, applied...)

	if err := saveJobsLocked(openPath, remaining); err != nil {
		return nil, nil, err
	}
	if err := saveJobsLocked(appliedPath, applied); err != nil {
		return nil, nil, err
	}
	return remaining, applied, nil
}

// ---- Apify run log ----
//
// Every actual (paid) Apify actor run is appended to a jobs_runs.json log. This
// is both the audit trail of "recent runs" and the source of truth for the
// rate-limit window: LastRunTime reads the newest entry so the handler can block
// a new run if one happened too recently.

// RunRecord is one Apify actor run.
type RunRecord struct {
	RanAt      time.Time `json:"ranAt"`
	JobsFound  int       `json:"jobsFound"`  // total jobs returned by the actor
	JobsAdded  int       `json:"jobsAdded"`  // new jobs merged into the open list
}

// runsMu guards concurrent read/modify/write of the runs log. It's separate from
// storeMu because the log is an independent file.
var runsMu sync.Mutex

func loadRunsLocked(path string) ([]RunRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunRecord{}, nil
		}
		return nil, err
	}
	var runs []RunRecord
	if err := json.Unmarshal(b, &runs); err != nil {
		return nil, err
	}
	if runs == nil {
		runs = []RunRecord{}
	}
	return runs, nil
}

// LoadRuns returns the recorded Apify runs, newest first. Missing file → empty.
func LoadRuns(runsPath string) ([]RunRecord, error) {
	runsMu.Lock()
	defer runsMu.Unlock()
	return loadRunsLocked(runsPath)
}

// LastRunTime returns when the most recent Apify run happened. The boolean is
// false if no run has ever been recorded.
func LastRunTime(runsPath string) (time.Time, bool) {
	runsMu.Lock()
	defer runsMu.Unlock()

	runs, err := loadRunsLocked(runsPath)
	if err != nil || len(runs) == 0 {
		return time.Time{}, false
	}
	return runs[0].RanAt, true // newest is first
}

// AppendRun records one Apify run (prepended so newest is first). The log is
// capped at the most recent maxRunLog entries so it never grows unbounded.
func AppendRun(runsPath string, rec RunRecord) error {
	runsMu.Lock()
	defer runsMu.Unlock()

	runs, err := loadRunsLocked(runsPath)
	if err != nil {
		return err
	}
	runs = append([]RunRecord{rec}, runs...)
	if len(runs) > maxRunLog {
		runs = runs[:maxRunLog]
	}
	b, err := json.MarshalIndent(runs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(runsPath, b, 0o644)
}

// maxRunLog bounds the size of the runs log.
const maxRunLog = 200
