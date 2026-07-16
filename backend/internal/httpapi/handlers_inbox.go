package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"emailsender/internal/email"
	"emailsender/internal/inbox"
	"emailsender/internal/resume"
)

// inboxCheckMin/Max bound how many recent messages a check scans.
const (
	inboxCheckMin     = 20
	inboxCheckMax     = 60
	inboxCheckDefault = 40
)

// handleRepliesCheck reads recent inbox mail, keeps replies to our outreach,
// AI-triages each, and merges them into the store. Slow (IMAP + N AI calls).
func (s *Server) handleRepliesCheck(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasInbox() {
		writeError(w, http.StatusBadRequest, "inbox reading not configured: set GMAIL_USER and GMAIL_APP_PASSWORD in backend/.env")
		return
	}
	var in struct {
		Limit int `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	limit := in.Limit
	if limit == 0 {
		limit = inboxCheckDefault
	}
	if limit < inboxCheckMin {
		limit = inboxCheckMin
	}
	if limit > inboxCheckMax {
		limit = inboxCheckMax
	}

	// Fetch recent inbox messages (read-only).
	msgs, err := inbox.FetchRecent(s.imapConfig(), limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Build the set of addresses we've emailed, from send history.
	history, _ := email.LoadHistory(s.cfg.HistoryPath)
	sentTo := make(map[string]bool, len(history))
	for _, h := range history {
		if e := strings.ToLower(strings.TrimSpace(h.RecipientEmail)); e != "" {
			sentTo[e] = true
		}
	}

	replies := inbox.MatchReplies(msgs, sentTo)

	// Profile (SD) gives the AI the candidate context for drafting.
	profile, _ := resume.LoadProfile(s.cfg.ProfilePath)

	// Triage each reply concurrently (small N). AI failure → safe default.
	records := make([]inbox.ReplyRecord, len(replies))
	var wg sync.WaitGroup
	for i := range replies {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := replies[i]
			v, _ := inbox.Classify(r.Context(), s.cfg.OpenAIKey, s.cfg.OpenAIModel, profile, m)
			rec := inbox.ReplyRecord{
				MessageID:  m.MessageID,
				FromEmail:  m.FromEmail,
				FromName:   m.FromName,
				Subject:    m.Subject,
				Body:       m.BodyText,
				ReceivedAt: m.Date,
				Category:   v.Category,
				Draft:      v.Draft,
				Reason:     v.Reason,
				Status:     inbox.StatusForVerdict(v),
				InReplyTo:  m.MessageID,
				References: m.References,
			}
			if v.Category == inbox.CatFollowUpLater && v.FollowUpInDays > 0 {
				t := time.Now().AddDate(0, 0, v.FollowUpInDays)
				rec.FollowUpAt = &t
			}
			records[i] = rec
		}(i)
	}
	wg.Wait()

	all, err := inbox.Merge(s.cfg.RepliesPath, records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save replies: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"replies": all,
		"checked": len(msgs),
		"matched": len(replies),
	})
}

// handleRepliesList returns the stored triaged replies.
func (s *Server) handleRepliesList(w http.ResponseWriter, r *http.Request) {
	all, err := inbox.LoadReplies(s.cfg.RepliesPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read replies: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"replies": all})
}

// handleReplySend sends the (edited) draft as a threaded reply and marks the
// record replied. No resume attachment — this is a conversation reply.
func (s *Server) handleReplySend(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasCredentials() {
		writeError(w, http.StatusBadRequest, "Gmail credentials missing: set GMAIL_USER and GMAIL_APP_PASSWORD in backend/.env")
		return
	}
	var in struct {
		MessageID string `json:"messageId"`
		Body      string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(in.MessageID) == "" || strings.TrimSpace(in.Body) == "" {
		writeError(w, http.StatusBadRequest, "messageId and body are required")
		return
	}

	rec, ok, err := inbox.Find(s.cfg.RepliesPath, in.MessageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "reply not found")
		return
	}

	profile, _ := resume.LoadProfile(s.cfg.ProfilePath)
	fromName := ""
	if profile != nil {
		fromName = profile.Name
	}

	subject := rec.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	if err := email.SendReply(s.smtpConfig(), fromName, rec.FromEmail, subject, in.Body, rec.MessageID, rec.References); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	all, err := inbox.SetStatus(s.cfg.RepliesPath, in.MessageID, inbox.StatusReplied)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "replies": all})
}

// handleReplyDismiss marks a reply as dismissed (no action).
func (s *Server) handleReplyDismiss(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MessageID string `json:"messageId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	all, err := inbox.SetStatus(s.cfg.RepliesPath, in.MessageID, inbox.StatusDismissed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "replies": all})
}
