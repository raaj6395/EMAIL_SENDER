package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"emailsender/internal/targets"
)

// handleTargetsList returns the cached target companies, filtered by comp floor,
// category, and a free-text query.
func (s *Server) handleTargetsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// targets.Load clamps anything below targets.FloorLPA, so a caller can raise
	// the bar but never lower it.
	minLPA := targets.FloorLPA
	if v := strings.TrimSpace(q.Get("minLPA")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > minLPA {
			minLPA = f
		}
	}

	var cats []string
	if v := strings.TrimSpace(q.Get("categories")); v != "" {
		for _, c := range strings.Split(v, ",") {
			if c = strings.TrimSpace(c); c != "" {
				cats = append(cats, c)
			}
		}
	}

	list, err := targets.Load(s.cfg.TargetsPath, targets.Filter{
		MinLPA:     minLPA,
		Categories: cats,
		Query:      q.Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the target list: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"companies": list,
		"total":     len(list),
		"cached":    targets.Count(s.cfg.TargetsPath),
		"aiEnabled": s.cfg.HasAI(),
		"floorLPA":  targets.FloorLPA,
		"minLPA":    minLPA,
	})
}

// handleTargetsDiscover asks the model for more companies matching the profile
// and merges the new ones into the cache.
func (s *Server) handleTargetsDiscover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MinLPA     float64  `json:"minLPA"`
		Categories []string `json:"categories"`
		Cities     []string `json:"cities"`
		Count      int      `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	added, err := targets.Discover(r.Context(), s.cfg.TargetsPath, s.cfg.OpenAIKey, s.cfg.OpenAIModel,
		targets.DiscoverOptions{
			MinLPA:     in.MinLPA,
			Categories: in.Categories,
			Cities:     in.Cities,
			Count:      in.Count,
		})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"added":  added,
		"count":  len(added),
		"cached": targets.Count(s.cfg.TargetsPath),
	})
}

// handleTargetsAdd scores one user-named company and stores it.
func (s *Server) handleTargetsAdd(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	c, err := targets.AddManual(r.Context(), s.cfg.TargetsPath, s.cfg.OpenAIKey, s.cfg.OpenAIModel, in.Name)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleTargetsRemove drops a company from the list.
func (s *Server) handleTargetsRemove(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := targets.Remove(s.cfg.TargetsPath, in.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
