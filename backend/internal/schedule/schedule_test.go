package schedule

import (
	"testing"
	"time"
)

// at builds a local time for a given weekday-ish date in August 2026.
// 2026-08-03 is a Monday, so day 3+n lines up with the weekday sequence.
func at(day, hour, min int) time.Time {
	return time.Date(2026, time.August, day, hour, min, 0, 0, time.Local)
}

func monWed11to13() Schedule {
	return Schedule{
		Enabled: true,
		Windows: []Window{
			{Day: time.Monday, StartMin: 11 * 60, EndMin: 13 * 60},
			{Day: time.Wednesday, StartMin: 11 * 60, EndMin: 13 * 60},
		},
		PerWindowCap: 40,
	}
}

func TestOpen(t *testing.T) {
	s := monWed11to13()
	cases := []struct {
		name string
		when time.Time
		want bool
	}{
		{"monday just before", at(3, 10, 59), false},
		{"monday at open", at(3, 11, 0), true},
		{"monday mid", at(3, 12, 30), true},
		{"monday at close is shut", at(3, 13, 0), false},
		{"tuesday noon", at(4, 12, 0), false},
		{"wednesday mid", at(5, 11, 30), true},
		{"friday", at(7, 12, 0), false},
		{"sunday", at(9, 12, 0), false},
	}
	for _, c := range cases {
		if got := s.Open(c.when); got != c.want {
			t.Errorf("%s: Open(%v) = %v, want %v", c.name, c.when, got, c.want)
		}
	}
}

func TestNextOpen(t *testing.T) {
	s := monWed11to13()
	cases := []struct {
		name string
		from time.Time
		want time.Time
	}{
		{"inside window returns now", at(3, 12, 0), at(3, 12, 0)},
		{"monday morning waits for 11", at(3, 8, 0), at(3, 11, 0)},
		{"monday after close waits for wednesday", at(3, 14, 0), at(5, 11, 0)},
		{"tuesday waits for wednesday", at(4, 9, 0), at(5, 11, 0)},
		{"wednesday after close wraps to next monday", at(5, 18, 0), at(10, 11, 0)},
		{"friday wraps to next monday", at(7, 12, 0), at(10, 11, 0)},
		{"sunday night wraps to monday", at(9, 23, 30), at(10, 11, 0)},
	}
	for _, c := range cases {
		got, ok := s.NextOpen(c.from)
		if !ok {
			t.Fatalf("%s: NextOpen(%v) not ok", c.name, c.from)
		}
		if !got.Equal(c.want) {
			t.Errorf("%s: NextOpen(%v) = %v, want %v", c.name, c.from, got, c.want)
		}
	}
}

func TestNextOpenNoWindows(t *testing.T) {
	s := Schedule{Enabled: true}
	if _, ok := s.NextOpen(at(3, 12, 0)); ok {
		t.Error("NextOpen with no windows should report not-ok")
	}
}

func TestDisabledScheduleAlwaysOpen(t *testing.T) {
	s := Schedule{Enabled: false}
	if !s.Open(at(7, 3, 0)) {
		t.Error("a disabled schedule should always be open")
	}
	got, ok := s.NextOpen(at(7, 3, 0))
	if !ok || !got.Equal(at(7, 3, 0)) {
		t.Errorf("disabled NextOpen = %v, %v; want the given time", got, ok)
	}
}

func TestCurrentWindowBounds(t *testing.T) {
	s := monWed11to13()

	start, ok := s.CurrentWindowStart(at(3, 12, 30))
	if !ok || !start.Equal(at(3, 11, 0)) {
		t.Errorf("CurrentWindowStart = %v, %v; want Monday 11:00", start, ok)
	}
	end, ok := s.CurrentWindowEnd(at(3, 12, 30))
	if !ok || !end.Equal(at(3, 13, 0)) {
		t.Errorf("CurrentWindowEnd = %v, %v; want Monday 13:00", end, ok)
	}
	if _, ok := s.CurrentWindowStart(at(4, 12, 0)); ok {
		t.Error("Tuesday should not be inside a window")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		s       Schedule
		wantErr bool
	}{
		{"default is valid", Default(), false},
		{"enabled with no windows", Schedule{Enabled: true}, true},
		{"disabled with no windows is fine", Schedule{Enabled: false}, false},
		{"end before start", Schedule{Enabled: true, Windows: []Window{{Day: time.Monday, StartMin: 780, EndMin: 660}}}, true},
		{"end equals start", Schedule{Enabled: true, Windows: []Window{{Day: time.Monday, StartMin: 660, EndMin: 660}}}, true},
		{"negative cap", Schedule{Enabled: true, Windows: []Window{{Day: time.Monday, StartMin: 660, EndMin: 780}}, PerWindowCap: -1}, true},
		{"end past midnight", Schedule{Enabled: true, Windows: []Window{{Day: time.Monday, StartMin: 660, EndMin: 1441}}}, true},
	}
	for _, c := range cases {
		err := c.s.Validate()
		if (err != nil) != c.wantErr {
			t.Errorf("%s: Validate() error = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}

func TestWindowLabel(t *testing.T) {
	w := Window{Day: time.Monday, StartMin: 11 * 60, EndMin: 13 * 60}
	if got := w.Label(); got != "Mon 11:00–13:00" {
		t.Errorf("Label() = %q", got)
	}
}

func TestParseHHMM(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"11:00", 660, false},
		{"9:30", 570, false},
		{"00:00", 0, false},
		{"23:59", 1439, false},
		{"nope", 0, true},
		{"11:75", 0, true},
	}
	for _, c := range cases {
		got, err := ParseHHMM(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseHHMM(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParseHHMM(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
