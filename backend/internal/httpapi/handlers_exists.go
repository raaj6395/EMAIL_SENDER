package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"

	"emailsender/internal/batch"
	"emailsender/internal/exists"
)

// existsEvent is one NDJSON line. A per-address line is written as soon as
// that address is checked. Confirmed ones are already in the send queue.
type existsEvent struct {
	Email   string `json:"email,omitempty"`
	Company string `json:"company,omitempty"`
	Name    string `json:"name,omitempty"`
	Kind    string `json:"kind,omitempty"` // exist, invalid, unknown
	Queued  bool   `json:"queued,omitempty"`
	Done    bool   `json:"done,omitempty"`
	Error   string `json:"error,omitempty"`
	Checked int    `json:"checked,omitempty"`
	Exist   int    `json:"exist,omitempty"`
	Invalid int    `json:"invalid,omitempty"`
	Unknown int    `json:"unknown,omitempty"`
	Added   int    `json:"added,omitempty"`
}

// handleExists streams a check of the pasted list. Each address the mail
// server confirms is added to the bulk queue before the next line is written.
func (s *Server) handleExists(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rows  string `json:"rows"`
		Track string `json:"track"`
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

	items := batch.ParseRows(in.Rows)
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "no valid emails found — paste one address per line (optionally 'email, Company, Name')")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not available")
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	var writeMu sync.Mutex
	enc := json.NewEncoder(w)
	send := func(ev existsEvent) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = enc.Encode(ev)
		flusher.Flush()
	}

	var mu sync.Mutex
	var checked, existN, invalidN, unknownN, added int
	err := exists.CheckAll(r.Context(), s.cfg.ReacherURL, toAddrs(items), func(a exists.Addr, kind string) {
		ev := existsEvent{Email: a.Email, Company: a.Company, Name: a.Name, Kind: kind}
		if kind == "exist" {
			n := s.batch.Add([]batch.Item{{Email: a.Email, Company: a.Company, Name: a.Name}}, track)
			ev.Queued = n > 0
			mu.Lock()
			existN++
			added += n
			checked++
			mu.Unlock()
		} else {
			mu.Lock()
			checked++
			if kind == "invalid" {
				invalidN++
			} else {
				unknownN++
			}
			mu.Unlock()
		}
		send(ev)
	})

	if err != nil && r.Context().Err() == nil {
		send(existsEvent{Error: err.Error()})
	}
	if r.Context().Err() == nil {
		send(existsEvent{
			Done: true, Checked: checked, Exist: existN, Invalid: invalidN, Unknown: unknownN, Added: added,
		})
	}
}

func toAddrs(items []batch.Item) []exists.Addr {
	addrs := make([]exists.Addr, len(items))
	for i, it := range items {
		addrs[i] = exists.Addr{Email: it.Email, Company: it.Company, Name: it.Name}
	}
	return addrs
}
