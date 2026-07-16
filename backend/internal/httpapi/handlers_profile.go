package httpapi

import (
	"encoding/json"
	"net/http"

	"emailsender/internal/resume"
)

func (s *Server) handleParseResume(w http.ResponseWriter, r *http.Request) {
	track := normalizeTrack(r.URL.Query().Get("track"))
	if !s.cfg.HasResumeFor(track) {
		name := "resume.pdf"
		if track == "ai" {
			name = "ai_resume.pdf"
		}
		writeError(w, http.StatusBadRequest, "resume not found: place your resume at backend/data/"+name)
		return
	}
	text, err := resume.ExtractText(s.cfg.ResumePathFor(track))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "could not read the PDF: "+err.Error())
		return
	}
	profile := resume.ParseProfile(text)

	// Merge any previously-saved edits (for this track) so a fresh parse doesn't
	// clobber the user's manual corrections. Saved non-empty fields win.
	if existing, _ := resume.LoadProfile(s.cfg.ProfilePathFor(track)); existing != nil {
		mergeProfile(profile, existing)
	}

	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	track := normalizeTrack(r.URL.Query().Get("track"))
	p, err := s.loadProfileForTrack(track)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read profile: "+err.Error())
		return
	}
	if p == nil {
		p = &resume.Profile{Skills: []string{}}
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	track := normalizeTrack(r.URL.Query().Get("track"))
	var p resume.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if p.Skills == nil {
		p.Skills = []string{}
	}
	if err := resume.SaveProfile(s.cfg.ProfilePathFor(track), &p); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save profile: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &p)
}
