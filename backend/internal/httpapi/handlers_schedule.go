package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"emailsender/internal/schedule"
)

// scheduleResponse is the schedule plus derived state the UI needs to explain
// what will happen next, without re-implementing the window math in the browser.
type scheduleResponse struct {
	Enabled      bool             `json:"enabled"`
	Windows      []windowResponse `json:"windows"`
	PerWindowCap int              `json:"perWindowCap"`
	Open         bool             `json:"open"`                   // a window is open right now
	NextWindowAt string           `json:"nextWindowAt,omitempty"` // RFC3339
	OpensInSec   int              `json:"opensInSec"`
	WindowEndsAt string           `json:"windowEndsAt,omitempty"`
	Timezone     string           `json:"timezone"` // server local zone, e.g. "IST"
}

type windowResponse struct {
	Day      int    `json:"day"`
	StartMin int    `json:"startMin"`
	EndMin   int    `json:"endMin"`
	Label    string `json:"label"`
}

// buildScheduleResponse decorates a schedule with the live "what happens next"
// fields.
func buildScheduleResponse(s schedule.Schedule) scheduleResponse {
	now := time.Now()
	zone, _ := now.Zone()

	out := scheduleResponse{
		Enabled:      s.Enabled,
		PerWindowCap: s.PerWindowCap,
		Open:         s.Open(now),
		Timezone:     zone,
		Windows:      make([]windowResponse, 0, len(s.Windows)),
	}
	for _, w := range s.Windows {
		out.Windows = append(out.Windows, windowResponse{
			Day: int(w.Day), StartMin: w.StartMin, EndMin: w.EndMin, Label: w.Label(),
		})
	}
	if end, open := s.CurrentWindowEnd(now); open {
		out.WindowEndsAt = end.Format(time.RFC3339)
	}
	if !out.Open {
		if next, ok := s.NextOpen(now); ok {
			out.NextWindowAt = next.Format(time.RFC3339)
			if d := time.Until(next); d > 0 {
				out.OpensInSec = int(d.Seconds()) + 1
			}
		}
	}
	return out
}

func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildScheduleResponse(s.sched.Get()))
}

// handleSaveSchedule replaces the weekly send windows and per-window cap. The
// queue worker is nudged afterwards so a newly-opened window starts draining
// immediately rather than on the next tick.
func (s *Server) handleSaveSchedule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled      bool `json:"enabled"`
		PerWindowCap int  `json:"perWindowCap"`
		Windows      []struct {
			Day      int `json:"day"`
			StartMin int `json:"startMin"`
			EndMin   int `json:"endMin"`
		} `json:"windows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	next := schedule.Schedule{Enabled: in.Enabled, PerWindowCap: in.PerWindowCap}
	for _, wd := range in.Windows {
		next.Windows = append(next.Windows, schedule.Window{
			Day:      time.Weekday(wd.Day),
			StartMin: wd.StartMin,
			EndMin:   wd.EndMin,
		})
	}

	if err := s.sched.Set(next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.batch.Notify()
	writeJSON(w, http.StatusOK, buildScheduleResponse(s.sched.Get()))
}
