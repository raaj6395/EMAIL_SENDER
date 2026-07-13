// Package jobs finds fresher-level software jobs in India by running the
// fantastic-jobs "advanced-linkedin-job-search-api" Apify actor. It uses
// Apify's "run-sync-get-dataset-items" endpoint, which runs the actor and
// returns its dataset in one blocking call — no polling. The actor supports
// server-side filtering, so we push the role/location/experience filters into
// the request rather than pulling global jobs and filtering in Go.
package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SearchConfig holds the Apify settings for the job-search actor.
type SearchConfig struct {
	Token   string
	ActorID string // e.g. "vIGxjRrHqDTPuE6M4" (fantastic-jobs/advanced-linkedin-job-search-api)
}

// Job is the normalized subset of an actor dataset item that the UI needs.
type Job struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Organization        string   `json:"organization"`
	OrganizationURL     string   `json:"organizationUrl"`
	OrganizationLogo    string   `json:"organizationLogo"`
	URL                 string   `json:"url"` // the LinkedIn apply link
	Location            string   `json:"location"`
	Seniority           string   `json:"seniority"`
	ExperienceLevel     string   `json:"experienceLevel"` // e.g. "0-2"
	EmploymentType      string   `json:"employmentType"`  // e.g. "FULL_TIME"
	WorkArrangement     string   `json:"workArrangement"` // e.g. "On-site" / "Remote"
	DatePosted          string   `json:"datePosted"`
	Description         string   `json:"description"`
	RequirementsSummary string   `json:"requirementsSummary"`
	KeySkills           []string `json:"keySkills"`
}

// DefaultRoles are the software roles the user wants when the request omits any.
var DefaultRoles = []string{
	"Software Engineer",
	"Software Engineer Intern",
	"Software Developer",
	"Backend Developer",
	"Backend Engineer",
}

// DefaultLocation is the location filter used when none is given.
const DefaultLocation = "India"

const apifyBase = "https://api.apify.com/v2/acts"

// Search runs the configured Apify actor filtered to the given roles + location
// and fresher (0-2 yr) experience, and returns normalized Jobs. A successful
// run that yields no jobs returns an empty slice with a nil error; only
// transport/auth/parse problems return an error.
func Search(ctx context.Context, cfg SearchConfig, roles []string, location string) ([]Job, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("Apify token not configured: set APIFY_TOKEN in backend/.env")
	}
	if len(roles) == 0 {
		roles = DefaultRoles
	}
	if strings.TrimSpace(location) == "" {
		location = DefaultLocation
	}

	actor := firstNonEmpty(cfg.ActorID, "vIGxjRrHqDTPuE6M4")
	// Apify accepts the actor id with "~" instead of "/" in the path; internal
	// actor IDs (e.g. "vIGxjRrHqDTPuE6M4") have no slash and pass through as-is.
	actorPath := strings.ReplaceAll(actor, "/", "~")

	// Server-side filters (verified against the live actor): timeRange "24h" gets
	// only jobs posted in the last day, titleSearch matches the roles,
	// locationSearch scopes to the country, and aiExperienceLevelFilter "0-2" is
	// the fresher bucket. limit is the per-call ceiling and must be >= 10.
	reqBody := map[string]any{
		"timeRange":               "24h",
		"limit":                   50,
		"descriptionType":         "text",
		"titleSearch":             roles,
		"locationSearch":          []string{location},
		"aiExperienceLevelFilter": []string{"0-2"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/run-sync-get-dataset-items?token=%s", apifyBase, actorPath, cfg.Token)

	// The actor scrapes and enriches jobs, so give it generous headroom.
	reqCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Apify request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Apify rejected the token (check APIFY_TOKEN)")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Apify actor %q not found (check APIFY_JOBS_ACTOR_ID)", actor)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Apify returned %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("could not parse Apify response: %w", err)
	}

	out := make([]Job, 0, len(items))
	for _, item := range items {
		j := Job{
			ID:                  getString(item, "id"),
			Title:               strings.TrimSpace(getString(item, "title")),
			Organization:        strings.TrimSpace(getString(item, "organization")),
			OrganizationURL:     getString(item, "organization_url"),
			OrganizationLogo:    getString(item, "organization_logo"),
			URL:                 getString(item, "url"),
			Location:            firstOfArray(item, "locations_derived"),
			Seniority:           strings.TrimSpace(getString(item, "seniority")),
			ExperienceLevel:     strings.TrimSpace(getString(item, "ai_experience_level")),
			EmploymentType:      firstOfArray(item, "employment_type"),
			WorkArrangement:     strings.TrimSpace(getString(item, "ai_work_arrangement")),
			DatePosted:          getString(item, "date_posted"),
			Description:         strings.TrimSpace(getString(item, "description_text")),
			RequirementsSummary: strings.TrimSpace(getString(item, "ai_requirements_summary")),
			KeySkills:           stringSlice(item, "ai_key_skills"),
		}
		// Skip malformed items with no usable identity or link.
		if j.ID == "" || j.URL == "" {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

// getString reads a key as a string, tolerating numbers/bools that some actors
// return (e.g. the numeric "id" field).
func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case bool:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// firstOfArray returns the first string element of an array-valued key (e.g.
// locations_derived, employment_type), or "" if absent/empty.
func firstOfArray(m map[string]any, key string) string {
	arr, ok := m[key].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	if s, ok := arr[0].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// stringSlice returns all string elements of an array-valued key.
func stringSlice(m map[string]any, key string) []string {
	arr, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
