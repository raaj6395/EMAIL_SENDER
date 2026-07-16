package httpapi

import "net/http"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"hasResume":      s.cfg.HasResume(),
		"hasResumeSD":    s.cfg.HasResumeFor("sd"),
		"hasResumeAI":    s.cfg.HasResumeFor("ai"),
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
