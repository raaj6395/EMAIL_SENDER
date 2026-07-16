package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"emailsender/internal/email"
	"emailsender/internal/inbox"
	"emailsender/internal/jobs"
	"emailsender/internal/lookup"
	"emailsender/internal/resume"
)

// ---- per-request config builders ----

func (s *Server) smtpConfig() email.SMTPConfig {
	return email.SMTPConfig{
		Host:     s.cfg.SMTPHost,
		Port:     s.cfg.SMTPPort,
		Username: s.cfg.GmailUser,
		Password: s.cfg.GmailAppPassword,
	}
}

func (s *Server) aiConfig() email.AIConfig {
	return email.AIConfig{
		APIKey: s.cfg.OpenAIKey,
		Model:  s.cfg.OpenAIModel,
	}
}

func (s *Server) lookupConfig() lookup.Config {
	return lookup.Config{
		Token:           s.cfg.ApifyToken,
		ActorID:         s.cfg.ApifyActorID,
		FallbackActorID: s.cfg.ApifyFallbackActorID,
		EmailField:      s.cfg.ApifyEmailField,
		NameField:       s.cfg.ApifyNameField,
		CompanyField:    s.cfg.ApifyCompanyField,
	}
}

func (s *Server) jobsConfig() jobs.SearchConfig {
	return jobs.SearchConfig{
		Token:   s.cfg.ApifyToken,
		ActorID: s.cfg.JobsActorID,
	}
}

func (s *Server) imapConfig() inbox.Config {
	return inbox.Config{
		Host:     s.cfg.IMAPHost,
		Port:     s.cfg.IMAPPort,
		Username: s.cfg.GmailUser,
		Password: s.cfg.GmailAppPassword,
	}
}

// ---- compose / profile helpers ----

// decodeComposeAndProfile parses the compose input, validates it, and loads the
// saved profile for the requested track. Writes an error response and returns
// ok=false on failure.
func (s *Server) decodeComposeAndProfile(w http.ResponseWriter, r *http.Request) (email.ComposeInput, *resume.Profile, bool) {
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
	in.Track = normalizeTrack(in.Track)

	p, err := s.loadProfileForTrack(in.Track)
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

// normalizeTrack coerces a track value to "sd" (default) or "ai".
func normalizeTrack(t string) string {
	if strings.ToLower(strings.TrimSpace(t)) == "ai" {
		return "ai"
	}
	return "sd"
}

// loadProfileForTrack loads a track's saved profile. For the AI track, if no
// profile has been saved yet, it seeds one from the SD profile so the user
// doesn't start blank (they can then edit / re-parse from the AI resume).
func (s *Server) loadProfileForTrack(track string) (*resume.Profile, error) {
	p, err := resume.LoadProfile(s.cfg.ProfilePathFor(track))
	if err != nil {
		return nil, err
	}
	if p == nil && track == "ai" {
		if base, _ := resume.LoadProfile(s.cfg.ProfilePath); base != nil {
			return base, nil
		}
	}
	return p, nil
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
