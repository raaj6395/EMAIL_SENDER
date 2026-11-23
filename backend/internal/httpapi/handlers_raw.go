package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"emailsender/internal/email"
	"emailsender/internal/verify"
)

// rawSendInput is a fully-written email: the caller supplies the subject and
// body instead of letting the template/AI compose them. Used by the MCP server
// so Claude can send exactly the text it wrote.
type rawSendInput struct {
	To         string `json:"to"`
	Subject    string `json:"subject"`
	BodyHTML   string `json:"bodyHTML"`
	BodyText   string `json:"bodyText"`
	Track      string `json:"track"`      // "sd" | "ai" — picks which resume to attach
	ResumePath string `json:"resumePath"` // optional; overrides the track's resume
	NoResume   bool   `json:"noResume"`   // send with no attachment at all
	Company    string `json:"company"`    // recorded in history only
}

// resolveAttachment decides which file to attach, if any, and validates it.
// Returns an empty path when the send should carry no attachment.
func (s *Server) resolveAttachment(in rawSendInput, track string) (string, error) {
	if in.NoResume {
		return "", nil
	}
	if p := strings.TrimSpace(in.ResumePath); p != "" {
		return p, validateResumePath(p)
	}
	// No explicit path: use the track's resume, which ValidateForSend checks.
	if err := s.cfg.ValidateForSend(track); err != nil {
		return "", err
	}
	return s.cfg.ResumePathFor(track), nil
}

// validateResumePath rejects anything that isn't an existing regular PDF. The
// path comes from an API caller, so it must not become a way to mail an
// arbitrary file off disk.
func validateResumePath(path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		return fmt.Errorf("resumePath must be a .pdf file")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("resumePath not found: %s", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("resumePath is not a regular file: %s", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("resumePath is empty: %s", path)
	}
	return nil
}

// handleSendRaw sends a caller-authored email. Unlike POST /api/send it does no
// composing — subject and body are used verbatim — but it keeps the same
// pre-send address check, resume attachment, and history logging.
func (s *Server) handleSendRaw(w http.ResponseWriter, r *http.Request) {
	var in rawSendInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	in.To = strings.TrimSpace(in.To)
	in.Subject = strings.TrimSpace(in.Subject)
	in.Company = strings.TrimSpace(in.Company)
	track := normalizeTrack(in.Track)

	if in.To == "" || !strings.Contains(in.To, "@") {
		writeError(w, http.StatusBadRequest, "a valid recipient email is required")
		return
	}
	if in.Subject == "" {
		writeError(w, http.StatusBadRequest, "subject is required")
		return
	}
	bodyHTML, bodyText := email.FillBodies(in.BodyHTML, in.BodyText)
	if strings.TrimSpace(bodyText) == "" {
		writeError(w, http.StatusBadRequest, "a body is required — set bodyText or bodyHTML")
		return
	}

	if !s.cfg.HasCredentials() {
		writeError(w, http.StatusBadRequest, "Gmail credentials missing: set GMAIL_USER and GMAIL_APP_PASSWORD in backend/.env")
		return
	}
	attachPath, err := s.resolveAttachment(in, track)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Same pre-send check as /api/send: don't burn a send on an address that
	// can't receive mail.
	if res := verify.Check(r.Context(), in.To); !res.Valid {
		writeError(w, http.StatusBadRequest, res.Reason)
		return
	}

	// The profile only supplies the sender's display name and attachment
	// filename here; a missing profile is not fatal for a raw send.
	fromName, attachmentName := "", "resume.pdf"
	if p, _ := s.loadProfileForTrack(track); p != nil {
		fromName = p.Name
		attachmentName = resumeAttachmentName(p.Name)
	}

	rendered := email.Rendered{
		Subject:        in.Subject,
		BodyHTML:       bodyHTML,
		BodyText:       bodyText,
		AttachmentName: attachmentName,
	}

	var sendErr error
	if attachPath == "" {
		sendErr = email.SendNoAttachment(s.smtpConfig(), fromName, in.To, rendered)
	} else {
		sendErr = email.Send(s.smtpConfig(), fromName, in.To, rendered, attachPath)
	}

	entry := email.HistoryEntry{
		RecipientEmail: in.To,
		Company:        in.Company,
		Subject:        in.Subject,
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
		"ok":         true,
		"subject":    in.Subject,
		"sentTo":     in.To,
		"source":     "raw",
		"attachment": attachmentNameOrNone(attachPath, attachmentName),
	})
}

func attachmentNameOrNone(path, name string) string {
	if path == "" {
		return ""
	}
	return name
}
