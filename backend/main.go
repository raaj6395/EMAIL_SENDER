package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"emailsender/internal/config"
	"emailsender/internal/email"
	"emailsender/internal/hr"
	"emailsender/internal/jobs"
	"emailsender/internal/lookup"
	"emailsender/internal/resume"
)

type server struct {
	cfg *config.Config
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	s := &server{cfg: cfg}

	// Seed a pre-filled profile on first run so the UI isn't blank.
	if existing, _ := resume.LoadProfile(cfg.ProfilePath); existing == nil {
		if err := resume.SaveProfile(cfg.ProfilePath, resume.DefaultProfile()); err != nil {
			log.Printf("warning: could not seed default profile: %v", err)
		} else {
			log.Printf("  seeded default profile at %s", cfg.ProfilePath)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/parse-resume", s.handleParseResume)
	mux.HandleFunc("GET /api/profile", s.handleGetProfile)
	mux.HandleFunc("PUT /api/profile", s.handleSaveProfile)
	mux.HandleFunc("POST /api/preview", s.handlePreview)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/digest", s.handleSendDigest)
	mux.HandleFunc("POST /api/lookup", s.handleLookup)
	mux.HandleFunc("POST /api/jobs/search", s.handleJobSearch)
	mux.HandleFunc("GET /api/jobs", s.handleJobsList)
	mux.HandleFunc("POST /api/jobs/applied", s.handleMarkApplied)
	mux.HandleFunc("GET /api/hr/whatsapp", s.handleHRWhatsApp)
	mux.HandleFunc("GET /api/hr/email", s.handleHREmail)
	mux.HandleFunc("POST /api/hr/rerank", s.handleHRRerank)

	handler := s.withCORS(mux)

	addr := ":" + cfg.Port
	log.Printf("email-sender backend listening on %s", addr)
	log.Printf("  data dir:     %s", cfg.DataDir)
	log.Printf("  resume found: %v", cfg.HasResume())
	log.Printf("  gmail creds:  %v", cfg.HasCredentials())
	log.Printf("  openai (ai):  %v (model %s)", cfg.HasAI(), cfg.OpenAIModel)
	log.Printf("  digest to:    %v", cfg.HasDigest())
	log.Printf("  apify lookup: %v (actor %s)", cfg.HasLookup(), cfg.ApifyActorID)
	log.Printf("  jobs search:  %v (actor %s)", cfg.HasJobs(), cfg.JobsActorID)
	log.Printf("  hr outreach:  %v", cfg.HasHR())
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// ---- middleware ----

func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- handlers ----

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"hasResume":      s.cfg.HasResume(),
		"hasCredentials": s.cfg.HasCredentials(),
		"gmailUser":      maskUser(s.cfg.GmailUser),
		"aiEnabled":      s.cfg.HasAI(),
		"aiModel":        s.cfg.OpenAIModel,
		"digestEnabled":  s.cfg.HasDigest(),
		"digestTo":       s.cfg.DigestTo,
		"lookupEnabled":  s.cfg.HasLookup(),
		"jobsEnabled":    s.cfg.HasJobs(),
		"hrEnabled":      s.cfg.HasHR(),
	})
}

func (s *server) handleParseResume(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasResume() {
		writeError(w, http.StatusBadRequest, "resume not found: place your resume at backend/data/resume.pdf")
		return
	}
	text, err := resume.ExtractText(s.cfg.ResumePath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "could not read the PDF: "+err.Error())
		return
	}
	profile := resume.ParseProfile(text)

	// Merge any previously-saved edits so we don't clobber the user's manual
	// corrections with a fresh parse. Saved non-empty fields win.
	if existing, _ := resume.LoadProfile(s.cfg.ProfilePath); existing != nil {
		mergeProfile(profile, existing)
	}

	writeJSON(w, http.StatusOK, profile)
}

func (s *server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	p, err := resume.LoadProfile(s.cfg.ProfilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read profile: "+err.Error())
		return
	}
	if p == nil {
		p = &resume.Profile{Skills: []string{}}
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	var p resume.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if err := resume.SaveProfile(s.cfg.ProfilePath, &p); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save profile: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &p)
}

func (s *server) handlePreview(w http.ResponseWriter, r *http.Request) {
	in, p, ok := s.decodeComposeAndProfile(w, r)
	if !ok {
		return
	}
	result := email.Compose(r.Context(), s.aiConfig(), p, in, resumeAttachmentName(p.Name))
	writeJSON(w, http.StatusOK, result)
}

func (s *server) handleSend(w http.ResponseWriter, r *http.Request) {
	in, p, ok := s.decodeComposeAndProfile(w, r)
	if !ok {
		return
	}
	if err := s.cfg.ValidateForSend(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	attachmentName := resumeAttachmentName(p.Name)
	result := email.Compose(r.Context(), s.aiConfig(), p, in, attachmentName)

	sendErr := email.Send(s.smtpConfig(), p.Name, in.RecipientEmail, result.Rendered, s.cfg.ResumePath)

	entry := email.HistoryEntry{
		RecipientEmail: in.RecipientEmail,
		Company:        in.Company,
		Subject:        result.Subject,
		Status:         "sent",
		SentAt:         time.Now(),
	}
	if sendErr != nil {
		entry.Status = "failed"
		entry.Error = sendErr.Error()
	}
	if histErr := email.AppendHistory(s.cfg.HistoryPath, entry); histErr != nil {
		log.Printf("warning: could not write history: %v", histErr)
	}

	if sendErr != nil {
		writeError(w, http.StatusBadGateway, sendErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"subject": result.Subject,
		"sentTo":  in.RecipientEmail,
		"source":  result.Source,
	})
}

func (s *server) handleSendDigest(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasDigest() {
		writeError(w, http.StatusBadRequest, "no digest recipient configured: set DIGEST_TO in backend/.env")
		return
	}
	if !s.cfg.HasCredentials() {
		writeError(w, http.StatusBadRequest, "Gmail credentials missing: set GMAIL_USER and GMAIL_APP_PASSWORD in backend/.env")
		return
	}

	entries, err := email.LoadHistory(s.cfg.HistoryPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read history: "+err.Error())
		return
	}

	if err := email.SendDigest(s.smtpConfig(), s.cfg.GmailUser, s.cfg.DigestTo, entries); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"sentTo": s.cfg.DigestTo,
		"count":  len(entries),
	})
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	entries, err := email.LoadHistory(s.cfg.HistoryPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read history: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *server) handleLookup(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasLookup() {
		writeError(w, http.StatusBadRequest, "LinkedIn lookup not configured: set APIFY_TOKEN in backend/.env")
		return
	}
	var in struct {
		LinkedInURL string `json:"linkedinUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	res, err := lookup.FindEmail(r.Context(), s.lookupConfig(), in.LinkedInURL)
	if err != nil {
		// A bad URL is a client error; anything else is an upstream/Apify failure.
		if errors.Is(err, lookup.ErrInvalidURL) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// A successful run with no email is not an error — report found:false so the
	// UI can prompt the user to enter the email manually.
	writeJSON(w, http.StatusOK, map[string]any{
		"found":      res.Found,
		"email":      res.Email,
		"name":       res.Name,
		"company":    res.Company,
		"confidence": res.Confidence,
		"status":     res.Status,
	})
}

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
func (s *server) handleJobSearch(w http.ResponseWriter, r *http.Request) {
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
func (s *server) handleJobsList(w http.ResponseWriter, r *http.Request) {
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
func (s *server) handleMarkApplied(w http.ResponseWriter, r *http.Request) {
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

// handleHRWhatsApp serves the WhatsApp contact sheet, company-ranked + paginated.
func (s *server) handleHRWhatsApp(w http.ResponseWriter, r *http.Request) {
	s.serveHRContacts(w, r, false)
}

// handleHREmail serves the email contact sheet, company-ranked + paginated.
func (s *server) handleHREmail(w http.ResponseWriter, r *http.Request) {
	s.serveHRContacts(w, r, true)
}

// serveHRContacts loads the requested sheet, ranks companies (cached), sorts,
// filters by an optional query, and returns one page of results.
func (s *server) serveHRContacts(w http.ResponseWriter, r *http.Request, isEmail bool) {
	if !s.cfg.HasHR() {
		writeError(w, http.StatusBadRequest, "HR data not found: place your HR spreadsheet at backend/data/HR DATA (1).xlsx")
		return
	}

	var (
		contacts []hr.Contact
		err      error
	)
	if isEmail {
		contacts, err = hr.LoadEmail(s.cfg.HRDataPath)
	} else {
		contacts, err = hr.LoadWhatsApp(s.cfg.HRDataPath)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read HR data: "+err.Error())
		return
	}

	// Rank companies (cached; only new companies hit the AI), then sort.
	ranks, err := hr.EnsureRanks(r.Context(), s.cfg.HRRanksPath, s.cfg.OpenAIKey, s.cfg.OpenAIModel, hr.UniqueCompanies(contacts), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not rank companies: "+err.Error())
		return
	}
	hr.ApplyRanksAndSort(contacts, ranks)

	// Optional search across company/name/role/email.
	if q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q"))); q != "" {
		filtered := contacts[:0]
		for _, c := range contacts {
			hay := strings.ToLower(c.Company + " " + c.Name + " " + c.Role + " " + c.Email)
			if strings.Contains(hay, q) {
				filtered = append(filtered, c)
			}
		}
		contacts = filtered
	}

	page, pageSize := paginationParams(r)
	total := len(contacts)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := contacts[start:end]
	if pageItems == nil {
		pageItems = []hr.Contact{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contacts": pageItems,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// handleHRRerank forces a fresh AI re-rank of all companies (both sheets).
func (s *server) handleHRRerank(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasHR() {
		writeError(w, http.StatusBadRequest, "HR data not found")
		return
	}
	wa, err := hr.LoadWhatsApp(s.cfg.HRDataPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	em, err := hr.LoadEmail(s.cfg.HRDataPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	companies := hr.UniqueCompanies(append(wa, em...))
	if _, err := hr.EnsureRanks(r.Context(), s.cfg.HRRanksPath, s.cfg.OpenAIKey, s.cfg.OpenAIModel, companies, true); err != nil {
		writeError(w, http.StatusInternalServerError, "could not re-rank: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "companies": len(companies)})
}

// paginationParams reads ?page= and ?pageSize= with sane defaults/limits.
func paginationParams(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 50
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("pageSize")); err == nil && v > 0 {
		pageSize = v
		if pageSize > 200 {
			pageSize = 200
		}
	}
	return page, pageSize
}

// ---- helpers ----

func (s *server) smtpConfig() email.SMTPConfig {
	return email.SMTPConfig{
		Host:     s.cfg.SMTPHost,
		Port:     s.cfg.SMTPPort,
		Username: s.cfg.GmailUser,
		Password: s.cfg.GmailAppPassword,
	}
}

func (s *server) aiConfig() email.AIConfig {
	return email.AIConfig{
		APIKey: s.cfg.OpenAIKey,
		Model:  s.cfg.OpenAIModel,
	}
}

func (s *server) lookupConfig() lookup.Config {
	return lookup.Config{
		Token:           s.cfg.ApifyToken,
		ActorID:         s.cfg.ApifyActorID,
		FallbackActorID: s.cfg.ApifyFallbackActorID,
		EmailField:      s.cfg.ApifyEmailField,
		NameField:       s.cfg.ApifyNameField,
		CompanyField:    s.cfg.ApifyCompanyField,
	}
}

func (s *server) jobsConfig() jobs.SearchConfig {
	return jobs.SearchConfig{
		Token:   s.cfg.ApifyToken,
		ActorID: s.cfg.JobsActorID,
	}
}

// decodeComposeAndProfile parses the compose input, validates it, and loads the
// saved profile. Writes an error response and returns ok=false on failure.
func (s *server) decodeComposeAndProfile(w http.ResponseWriter, r *http.Request) (email.ComposeInput, *resume.Profile, bool) {
	var in email.ComposeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return in, nil, false
	}
	in.RecipientEmail = strings.TrimSpace(in.RecipientEmail)
	in.RecipientName = strings.TrimSpace(in.RecipientName)
	in.Company = strings.TrimSpace(in.Company)
	if in.RecipientEmail == "" || !strings.Contains(in.RecipientEmail, "@") {
		writeError(w, http.StatusBadRequest, "a valid recipient email is required")
		return in, nil, false
	}
	if in.Company == "" {
		writeError(w, http.StatusBadRequest, "company name is required")
		return in, nil, false
	}

	p, err := resume.LoadProfile(s.cfg.ProfilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read profile: "+err.Error())
		return in, nil, false
	}
	if p == nil {
		writeError(w, http.StatusBadRequest, "no profile saved yet — parse your resume and save your profile first")
		return in, nil, false
	}
	return in, p, true
}

// mergeProfile fills dst's empty fields from src (saved edits take precedence
// only where they are non-empty).
func mergeProfile(dst, src *resume.Profile) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Email != "" {
		dst.Email = src.Email
	}
	if src.Phone != "" {
		dst.Phone = src.Phone
	}
	if src.TargetRole != "" {
		dst.TargetRole = src.TargetRole
	}
	if len(src.Skills) > 0 {
		dst.Skills = src.Skills
	}
	if src.Pitch != "" {
		dst.Pitch = src.Pitch
	}
	if src.LinkedIn != "" {
		dst.LinkedIn = src.LinkedIn
	}
	if src.GitHub != "" {
		dst.GitHub = src.GitHub
	}
	if src.Portfolio != "" {
		dst.Portfolio = src.Portfolio
	}
}

// resumeAttachmentName produces a professional filename like "Ankit_Raj_Resume.pdf".
func resumeAttachmentName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "resume.pdf"
	}
	safe := strings.NewReplacer(" ", "_", "/", "_", "\\", "_").Replace(name)
	return safe + "_Resume.pdf"
}

func maskUser(u string) string {
	if u == "" {
		return ""
	}
	parts := strings.SplitN(u, "@", 2)
	local := parts[0]
	if len(local) <= 2 {
		local = local + "***"
	} else {
		local = local[:2] + "***"
	}
	if len(parts) == 2 {
		return local + "@" + parts[1]
	}
	return local
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}
