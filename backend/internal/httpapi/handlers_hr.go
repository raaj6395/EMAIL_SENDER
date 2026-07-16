package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"emailsender/internal/hr"
)

// handleHRWhatsApp serves the WhatsApp contact sheet, company-ranked + paginated.
func (s *Server) handleHRWhatsApp(w http.ResponseWriter, r *http.Request) {
	s.serveHRContacts(w, r, false)
}

// handleHREmail serves the email contact sheet, company-ranked + paginated.
func (s *Server) handleHREmail(w http.ResponseWriter, r *http.Request) {
	s.serveHRContacts(w, r, true)
}

// serveHRContacts loads the requested sheet, ranks companies (cached), sorts,
// filters by an optional query, and returns one page of results.
func (s *Server) serveHRContacts(w http.ResponseWriter, r *http.Request, isEmail bool) {
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

	// Hide contacts already reached out to on this channel — they belong in the
	// Sent section instead.
	channel := "whatsapp"
	if isEmail {
		channel = "email"
	}
	sentKeys, err := hr.SentKeys(s.cfg.HRSentPath, channel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read sent list: "+err.Error())
		return
	}
	if len(sentKeys) > 0 {
		kept := contacts[:0]
		for _, c := range contacts {
			if !sentKeys[c.Key()] {
				kept = append(kept, c)
			}
		}
		contacts = kept
	}

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

	// Sent list for this channel (full, newest first — small enough to send whole).
	sent, err := s.hrSentForChannel(channel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read sent list: "+err.Error())
		return
	}

	resp := map[string]any{
		"contacts": pageItems,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"sent":     sent,
	}
	// WhatsApp also reports the current send-rate status so the UI can show the
	// cooldown countdown / daily-cap state on load.
	if !isEmail {
		if rs, err := hr.WhatsAppRateStatus(s.cfg.HRSentPath); err == nil {
			resp["rate"] = rs
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// hrSentForChannel returns the sent records for one channel, newest first.
func (s *Server) hrSentForChannel(channel string) ([]hr.SentRecord, error) {
	all, err := hr.LoadSent(s.cfg.HRSentPath)
	if err != nil {
		return nil, err
	}
	out := make([]hr.SentRecord, 0, len(all))
	for _, rec := range all {
		if rec.Channel == channel {
			out = append(out, rec)
		}
	}
	return out, nil
}

// handleHRMarkSent records a contact as reached out to. Body carries the
// contact fields + channel. Returns the updated sent list for that channel.
func (s *Server) handleHRMarkSent(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasHR() {
		writeError(w, http.StatusBadRequest, "HR data not found")
		return
	}
	var in struct {
		Channel string `json:"channel"` // "email" | "whatsapp"
		Company string `json:"company"`
		Name    string `json:"name"`
		Role    string `json:"role"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Key     string `json:"key"` // optional explicit key; else derived
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if in.Channel != "email" && in.Channel != "whatsapp" {
		writeError(w, http.StatusBadRequest, "channel must be 'email' or 'whatsapp'")
		return
	}
	key := strings.TrimSpace(strings.ToLower(in.Key))
	if key == "" {
		if e := strings.TrimSpace(strings.ToLower(in.Email)); e != "" {
			key = e
		} else {
			key = strings.TrimSpace(in.Phone)
		}
	}
	if key == "" {
		writeError(w, http.StatusBadRequest, "a contact key (email or phone) is required")
		return
	}

	// Enforce WhatsApp send-rate limits server-side so a fast/over-cap send is
	// actually prevented (not just discouraged in the UI) — this is the real
	// guard against getting the number flagged.
	if in.Channel == "whatsapp" {
		rs, err := hr.WhatsAppRateStatus(s.cfg.HRSentPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not check send rate: "+err.Error())
			return
		}
		if rs.Blocked {
			msg := ""
			if rs.CapReached {
				mins := (rs.ResetIn + 59) / 60
				msg = fmt.Sprintf("WhatsApp limit reached (%d in %dh). Pause ~%d min so you don't get flagged.", rs.WindowCap, rs.WindowHours, mins)
			} else {
				msg = fmt.Sprintf("Slow down — wait %ds before the next WhatsApp message to avoid getting flagged.", rs.CooldownLeft)
			}
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": msg, "rate": rs})
			return
		}
	}

	if _, err := hr.MarkSent(s.cfg.HRSentPath, hr.SentRecord{
		Key:     key,
		Channel: in.Channel,
		Company: in.Company,
		Name:    in.Name,
		Role:    in.Role,
		Email:   in.Email,
		Phone:   in.Phone,
		SentAt:  time.Now(),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record sent: "+err.Error())
		return
	}
	sent, err := s.hrSentForChannel(in.Channel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{"sent": sent}
	if in.Channel == "whatsapp" {
		if rs, err := hr.WhatsAppRateStatus(s.cfg.HRSentPath); err == nil {
			out["rate"] = rs // fresh status so the UI starts the cooldown immediately
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleHRRerank forces a fresh AI re-rank of all companies (both sheets).
func (s *Server) handleHRRerank(w http.ResponseWriter, r *http.Request) {
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
