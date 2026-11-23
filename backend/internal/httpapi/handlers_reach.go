package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"emailsender/internal/batch"
	"emailsender/internal/hr"
	"emailsender/internal/reach"
)

// reachChannel is the channel name used in the shared sent log, so contacts
// emailed from the Reach page disappear from it on the next load.
const reachChannel = "reach"

// Per-request work caps. The pipeline runs incrementally: each verify/screen
// call does a bounded chunk so an HTTP request never hangs on 540 rows, and
// repeated calls walk through the sheet.
const (
	defaultVerifyLimit = 120
	defaultScreenLimit = 50
)

// reachRows loads and parses the sheet, or writes the "not configured" error.
func (s *Server) reachRows(w http.ResponseWriter) ([]reach.Row, bool) {
	if !s.cfg.HasReach() {
		writeError(w, http.StatusBadRequest, "Reach data not found: place your outreach spreadsheet at backend/data/reach.xlsx")
		return nil, false
	}
	rows, err := reach.LoadRows(s.cfg.ReachDataPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read reach data: "+err.Error())
		return nil, false
	}
	return rows, true
}

// handleReachList serves the shortlist: sheet rows joined with their cached
// verification + screening results, filtered and paginated. It never blocks on
// the pipeline — whatever is cached is what you see, and the progress counters
// tell the UI how much is still pending.
func (s *Server) handleReachList(w http.ResponseWriter, r *http.Request) {
	rows, ok := s.reachRows(w)
	if !ok {
		return
	}
	verdicts, screens, err := reach.Load(s.cfg.ReachCachePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read reach cache: "+err.Error())
		return
	}

	sentKeys, err := hr.SentKeys(s.cfg.HRSentPath, reachChannel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read sent list: "+err.Error())
		return
	}

	f := reach.Filter{
		Query:     r.URL.Query().Get("q"),
		OnlyReady: r.URL.Query().Get("all") != "1", // default: only ready-to-send
		Exclude:   sentKeys,
	}
	if v, err := strconv.ParseFloat(r.URL.Query().Get("minLPA"), 64); err == nil {
		f.MinLPA = v
	}

	contacts, prog := reach.Build(rows, verdicts, screens, f)

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
	items := contacts[start:end]
	if items == nil {
		items = []reach.Contact{}
	}

	sent, err := s.hrSentForChannel(reachChannel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contacts": items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"progress": prog,
		"sent":     sent,
		"floorLPA": reach.FloorLPA,
	})
}

// handleReachVerify runs the next chunk of email deliverability checks.
func (s *Server) handleReachVerify(w http.ResponseWriter, r *http.Request) {
	rows, ok := s.reachRows(w)
	if !ok {
		return
	}
	limit := bodyLimit(r, defaultVerifyLimit)

	checked, valid, err := reach.VerifyBatch(r.Context(), s.cfg.ReachCachePath, rows, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "verification failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"checked": checked,
		"valid":   valid,
		"bad":     checked - valid,
	})
}

// handleReachScreen screens the next chunk of companies for the fresher-SDE
// 12+ LPA bar.
func (s *Server) handleReachScreen(w http.ResponseWriter, r *http.Request) {
	rows, ok := s.reachRows(w)
	if !ok {
		return
	}
	limit := bodyLimit(r, defaultScreenLimit)

	screened, qualified, err := reach.ScreenBatch(r.Context(), s.cfg.ReachCachePath,
		s.cfg.OpenAIKey, s.cfg.OpenAIModel, reach.Companies(rows), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "screening failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"screened":  screened,
		"qualified": qualified,
	})
}

// handleReachQueue adds selected contacts to the scheduled send queue, so they
// go out gradually in the configured windows instead of all at once.
func (s *Server) handleReachQueue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Track    string `json:"track"`
		Contacts []struct {
			Email   string `json:"email"`
			Company string `json:"company"`
			Name    string `json:"name"`
		} `json:"contacts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	track := normalizeTrack(in.Track)
	if err := s.cfg.ValidateForSend(track); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items := make([]batch.Item, 0, len(in.Contacts))
	for _, c := range in.Contacts {
		email := strings.TrimSpace(strings.ToLower(c.Email))
		if email == "" {
			continue
		}
		items = append(items, batch.Item{Email: email, Company: c.Company, Name: c.Name})
	}
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "no contacts to queue")
		return
	}

	added := s.batch.Add(items, track)

	// Queued contacts are recorded as reached out to so they drop off the list
	// immediately — otherwise you'd queue the same person again on the next load.
	for _, c := range in.Contacts {
		email := strings.TrimSpace(strings.ToLower(c.Email))
		if email == "" {
			continue
		}
		if _, err := hr.MarkSent(s.cfg.HRSentPath, hr.SentRecord{
			Key: email, Channel: reachChannel, Company: c.Company,
			Name: c.Name, Email: email, SentAt: time.Now(),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "queued, but could not record it: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"added":   added,
		"skipped": len(items) - added,
		"queue":   s.batch.Get(),
	})
}

// bodyLimit reads an optional {"limit": n} from the request body.
func bodyLimit(r *http.Request, def int) int {
	var in struct {
		Limit int `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err == nil && in.Limit > 0 {
		return in.Limit
	}
	return def
}
