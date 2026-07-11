package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"emailsender/internal/config"
	"emailsender/internal/email"
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

	handler := s.withCORS(mux)

	addr := ":" + cfg.Port
	log.Printf("email-sender backend listening on %s", addr)
	log.Printf("  data dir:     %s", cfg.DataDir)
	log.Printf("  resume found: %v", cfg.HasResume())
	log.Printf("  gmail creds:  %v", cfg.HasCredentials())
	log.Printf("  openai (ai):  %v (model %s)", cfg.HasAI(), cfg.OpenAIModel)
	log.Printf("  digest to:    %v", cfg.HasDigest())
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
