// Package schedule defines the weekly send windows that gate outgoing bulk
// email. Cold outreach lands better when it arrives during working hours, so
// the queue only drains inside these windows (e.g. Mon 11:00–13:00 and Wed
// 11:00–13:00) and holds everything else until the next one opens.
//
// Windows are evaluated in server local time and persisted to JSON so the
// schedule survives a restart.
package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Window is one recurring weekly slot: a day plus a half-open local-time range
// [StartMin, EndMin) expressed as minutes from midnight.
type Window struct {
	Day      time.Weekday `json:"day"`      // 0=Sunday … 6=Saturday
	StartMin int          `json:"startMin"` // minutes from midnight, inclusive
	EndMin   int          `json:"endMin"`   // minutes from midnight, exclusive
}

// Schedule is the full weekly plan plus the per-window send cap.
type Schedule struct {
	Enabled bool     `json:"enabled"` // false → the queue drains immediately, ignoring windows
	Windows []Window `json:"windows"`
	// PerWindowCap limits how many emails may be sent within a single window.
	// Leftovers roll over to the next window. 0 means no cap.
	PerWindowCap int `json:"perWindowCap"`
}

// Default is the schedule the app starts with: Monday and Wednesday, 11:00 to
// 13:00 local time, capped at 40 sends per window.
func Default() Schedule {
	return Schedule{
		Enabled: true,
		Windows: []Window{
			{Day: time.Monday, StartMin: 11 * 60, EndMin: 13 * 60},
			{Day: time.Wednesday, StartMin: 11 * 60, EndMin: 13 * 60},
		},
		PerWindowCap: 40,
	}
}

// Validate checks the schedule is coherent, returning a human-readable error.
func (s Schedule) Validate() error {
	if s.PerWindowCap < 0 {
		return errors.New("per-window cap cannot be negative")
	}
	if s.Enabled && len(s.Windows) == 0 {
		return errors.New("add at least one send window, or turn the schedule off")
	}
	for _, w := range s.Windows {
		if w.Day < time.Sunday || w.Day > time.Saturday {
			return fmt.Errorf("invalid day %d", int(w.Day))
		}
		if w.StartMin < 0 || w.StartMin >= 24*60 {
			return fmt.Errorf("%s: start time out of range", w.Day)
		}
		if w.EndMin <= w.StartMin || w.EndMin > 24*60 {
			return fmt.Errorf("%s: end time must be after the start time", w.Day)
		}
	}
	return nil
}

// normalized returns the windows sorted by day then start, which makes the
// "next window" scan below a simple forward walk.
func (s Schedule) normalized() []Window {
	out := make([]Window, len(s.Windows))
	copy(out, s.Windows)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].StartMin < out[j].StartMin
	})
	return out
}

// minutesOfDay is t's local wall-clock time as minutes from midnight.
func minutesOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

// Contains reports whether t falls inside this window (local time).
func (w Window) Contains(t time.Time) bool {
	if t.Weekday() != w.Day {
		return false
	}
	m := minutesOfDay(t)
	return m >= w.StartMin && m < w.EndMin
}

// startOn returns the window's start instant on the local day containing t.
func (w Window) startOn(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return d.Add(time.Duration(w.StartMin) * time.Minute)
}

// endOn returns the window's end instant on the local day containing t.
func (w Window) endOn(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return d.Add(time.Duration(w.EndMin) * time.Minute)
}

// Label renders the window for the UI, e.g. "Mon 11:00–13:00".
func (w Window) Label() string {
	return fmt.Sprintf("%s %s–%s", w.Day.String()[:3], hhmm(w.StartMin), hhmm(w.EndMin))
}

func hhmm(min int) string {
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// Open reports whether t is inside any window. A disabled schedule is always
// open — that's the "send immediately" escape hatch.
func (s Schedule) Open(t time.Time) bool {
	if !s.Enabled {
		return true
	}
	for _, w := range s.Windows {
		if w.Contains(t) {
			return true
		}
	}
	return false
}

// CurrentWindowStart returns the start instant of the window containing t, and
// whether t is inside one. It is the identity of "this window" — the queue uses
// it to scope the per-window send cap.
func (s Schedule) CurrentWindowStart(t time.Time) (time.Time, bool) {
	if !s.Enabled {
		return time.Time{}, false
	}
	for _, w := range s.Windows {
		if w.Contains(t) {
			return w.startOn(t), true
		}
	}
	return time.Time{}, false
}

// CurrentWindowEnd returns the end instant of the window containing t.
func (s Schedule) CurrentWindowEnd(t time.Time) (time.Time, bool) {
	if !s.Enabled {
		return time.Time{}, false
	}
	for _, w := range s.Windows {
		if w.Contains(t) {
			return w.endOn(t), true
		}
	}
	return time.Time{}, false
}

// NextOpen returns the next instant at or after t when a window is open. If t is
// already inside a window, t itself is returned. ok is false when the schedule
// has no windows to open (so the caller can report "never").
//
// It scans the next 8 days, which is enough to find any weekly window.
func (s Schedule) NextOpen(t time.Time) (time.Time, bool) {
	if !s.Enabled {
		return t, true
	}
	if len(s.Windows) == 0 {
		return time.Time{}, false
	}
	if s.Open(t) {
		return t, true
	}

	windows := s.normalized()
	// Walk day by day; within each day take the earliest window that starts at or
	// after the current moment.
	for dayOffset := 0; dayOffset < 8; dayOffset++ {
		day := t.AddDate(0, 0, dayOffset)
		for _, w := range windows {
			if w.Day != day.Weekday() {
				continue
			}
			start := w.startOn(day)
			if !start.Before(t) {
				return start, true
			}
		}
	}
	return time.Time{}, false
}

// ---- persistence ----

// Store is a JSON-file-backed schedule, safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	path string
	cur  Schedule
}

// LoadStore reads the schedule from path, falling back to Default() when the
// file is missing or unreadable. A malformed file is reported as an error but
// the store still comes up on the default schedule so sending is never wedged.
func LoadStore(path string) (*Store, error) {
	st := &Store{path: path, cur: Default()}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	var s Schedule
	if err := json.Unmarshal(b, &s); err != nil {
		return st, fmt.Errorf("schedule file is malformed, using defaults: %w", err)
	}
	if err := s.Validate(); err != nil {
		return st, fmt.Errorf("schedule file is invalid, using defaults: %w", err)
	}
	st.cur = s
	return st, nil
}

// Get returns a copy of the current schedule.
func (st *Store) Get() Schedule {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := st.cur
	out.Windows = append([]Window(nil), st.cur.Windows...)
	return out
}

// Set validates and persists a new schedule.
func (st *Store) Set(s Schedule) error {
	if err := s.Validate(); err != nil {
		return err
	}
	s.Windows = dedupeWindows(s.Windows)

	st.mu.Lock()
	defer st.mu.Unlock()
	st.cur = s

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(st.path, b, 0o644)
}

// dedupeWindows drops exact duplicates and sorts, so saving the same window
// twice from the UI doesn't grow the list.
func dedupeWindows(ws []Window) []Window {
	seen := map[Window]bool{}
	out := make([]Window, 0, len(ws))
	for _, w := range ws {
		if seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].StartMin < out[j].StartMin
	})
	return out
}

// ParseHHMM converts "11:00" / "9:30" to minutes from midnight.
func ParseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, fmt.Errorf("invalid time %q (use HH:MM)", s)
	}
	if h < 0 || h > 24 || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return h*60 + m, nil
}
