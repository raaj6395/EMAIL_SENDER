package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"emailsender/internal/lookup"
)

func (s *Server) handleLookup(w http.ResponseWriter, r *http.Request) {
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
