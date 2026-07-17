package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"emailsender/internal/batch"
	"emailsender/internal/email"
)

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	in, p, ok := s.decodeComposeAndProfile(w, r)
	if !ok {
		return
	}
	result := email.Compose(r.Context(), s.aiConfig(), p, in, resumeAttachmentName(p.Name))
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	in, p, ok := s.decodeComposeAndProfile(w, r)
	if !ok {
		return
	}
	if err := s.cfg.ValidateForSend(in.Track); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	attachmentName := resumeAttachmentName(p.Name)
	result := email.Compose(r.Context(), s.aiConfig(), p, in, attachmentName)

	// Attach the resume for the selected track (SD → resume.pdf, AI → ai_resume.pdf).
	sendErr := email.Send(s.smtpConfig(), p.Name, in.RecipientEmail, result.Rendered, s.cfg.ResumePathFor(in.Track))

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

// batchMaxGapSec is the upper bound (seconds) of the random gap between bulk
// sends — a random 0..N pause keeps Gmail from seeing a rapid burst.
const batchMaxGapSec = 20

// sendOneBatch is the per-item worker for a bulk batch: it composes an
// AI-tailored email for the item's company (on the given track), sends it with
// that track's resume, and records the result in history — mirroring a single
// send. Returned errors mark the item failed.
func (s *Server) sendOneBatch(ctx context.Context, track string, it batch.Item) error {
	p, err := s.loadProfileForTrack(track)
	if err != nil || p == nil {
		return fmt.Errorf("no %s profile saved", strings.ToUpper(track))
	}

	in := email.ComposeInput{
		RecipientEmail: it.Email,
		RecipientName:  it.Name,
		Company:        it.Company,
		Track:          track,
	}
	result := email.Compose(ctx, s.aiConfig(), p, in, resumeAttachmentName(p.Name))
	sendErr := email.Send(s.smtpConfig(), p.Name, it.Email, result.Rendered, s.cfg.ResumePathFor(track))

	entry := email.HistoryEntry{
		RecipientEmail: it.Email,
		Company:        it.Company,
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
	return sendErr
}

// handleBatchStart parses pasted rows and starts a paced bulk send. Only one
// batch runs at a time.
func (s *Server) handleBatchStart(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Track string `json:"track"`
		Rows  string `json:"rows"` // pasted text, one recipient per line
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
	if s.batch.Running() {
		writeError(w, http.StatusConflict, "a bulk send is already in progress — wait for it to finish or cancel it")
		return
	}

	items := batch.ParseRows(in.Rows)
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "no valid recipients found — paste one email per line (optionally 'email, Company, Name')")
		return
	}

	if !s.batch.Start(items, track, batchMaxGapSec, s.sendOneBatch) {
		writeError(w, http.StatusConflict, "a bulk send is already in progress")
		return
	}
	writeJSON(w, http.StatusOK, s.batch.Get())
}

// handleBatchStatus returns the current/last batch progress for polling.
func (s *Server) handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.batch.Get())
}

// handleBatchPause holds the queue before the next send (Stop).
func (s *Server) handleBatchPause(w http.ResponseWriter, r *http.Request) {
	s.batch.Pause()
	writeJSON(w, http.StatusOK, s.batch.Get())
}

// handleBatchResume continues a paused queue.
func (s *Server) handleBatchResume(w http.ResponseWriter, r *http.Request) {
	s.batch.Resume()
	writeJSON(w, http.StatusOK, s.batch.Get())
}

// handleBatchCancel aborts the batch; remaining items are skipped.
func (s *Server) handleBatchCancel(w http.ResponseWriter, r *http.Request) {
	s.batch.Cancel()
	writeJSON(w, http.StatusOK, s.batch.Get())
}

func (s *Server) handleSendDigest(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	entries, err := email.LoadHistory(s.cfg.HistoryPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read history: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
