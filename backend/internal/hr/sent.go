package hr

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// SentRecord is one contact you've already reached out to.
type SentRecord struct {
	Key     string    `json:"key"`     // contact key (email or waPhone)
	Channel string    `json:"channel"` // "email" | "whatsapp"
	Company string    `json:"company"`
	Name    string    `json:"name"`
	Role    string    `json:"role"`
	Email   string    `json:"email,omitempty"`
	Phone   string    `json:"phone,omitempty"`
	SentAt  time.Time `json:"sentAt"`
}

// Key is the stable identity of a contact for sent-tracking: the email for
// email contacts, else the wa.me phone. Lowercased so matching is consistent.
func (c Contact) Key() string {
	if e := strings.TrimSpace(strings.ToLower(c.Email)); e != "" {
		return e
	}
	return strings.TrimSpace(c.WaPhone)
}

// sentMu guards read/modify/write of the sent-contacts file.
var sentMu sync.Mutex

func loadSentLocked(path string) ([]SentRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SentRecord{}, nil
		}
		return nil, err
	}
	var recs []SentRecord
	if err := json.Unmarshal(b, &recs); err != nil {
		return nil, err
	}
	if recs == nil {
		recs = []SentRecord{}
	}
	return recs, nil
}

// LoadSent returns all sent records, newest first. Missing file → empty slice.
func LoadSent(path string) ([]SentRecord, error) {
	sentMu.Lock()
	defer sentMu.Unlock()
	return loadSentLocked(path)
}

// SentKeys returns the set of contact keys already marked sent, optionally
// filtered to a single channel ("email"/"whatsapp"; "" = any).
func SentKeys(path, channel string) (map[string]bool, error) {
	sentMu.Lock()
	defer sentMu.Unlock()
	recs, err := loadSentLocked(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(recs))
	for _, r := range recs {
		if channel == "" || r.Channel == channel {
			out[r.Key] = true
		}
	}
	return out, nil
}

// WhatsApp send-rate limit: a conservative cap for a new/unwarmed number,
// counted over a rolling 6-hour window (not a calendar day) so sends are spread
// out. There is intentionally NO per-message cooldown — the window cap alone is
// the guard; you can send messages back-to-back until the cap is reached.
const (
	WhatsAppWindowCap = 15
	WhatsAppWindow    = 6 * time.Hour
)

// RateStatus describes whether a new send is currently allowed for a channel.
type RateStatus struct {
	SentInWindow int  `json:"sentInWindow"` // sends within the rolling window
	WindowCap    int  `json:"windowCap"`    // max sends allowed per window
	WindowHours  int  `json:"windowHours"`  // the window length, in hours
	CooldownLeft int  `json:"cooldownLeft"` // seconds until the inter-send cooldown clears
	ResetIn      int  `json:"resetIn"`      // seconds until the cap frees up (oldest send ages out)
	Blocked      bool `json:"blocked"`      // true if a send is not allowed right now
	CapReached   bool `json:"capReached"`   // true if the window cap is hit
}

// WhatsAppRateStatus computes the current WhatsApp send-rate status from the
// recorded sent timestamps: how many were sent in the last WhatsAppWindow, how
// long until the inter-send cooldown clears, and how long until the cap frees up.
func WhatsAppRateStatus(path string) (RateStatus, error) {
	sentMu.Lock()
	defer sentMu.Unlock()
	recs, err := loadSentLocked(path)
	if err != nil {
		return RateStatus{}, err
	}

	now := time.Now()
	windowStart := now.Add(-WhatsAppWindow)

	// Collect WhatsApp send times inside the rolling window.
	var inWindow []time.Time
	for _, r := range recs {
		if r.Channel != "whatsapp" {
			continue
		}
		if r.SentAt.After(windowStart) {
			inWindow = append(inWindow, r.SentAt)
		}
	}

	capReached := len(inWindow) >= WhatsAppWindowCap

	// resetIn: when the cap is hit, one slot frees up once the OLDEST in-window
	// send ages past the window. Find the oldest of the in-window sends.
	resetIn := 0
	if capReached && len(inWindow) > 0 {
		oldest := inWindow[0]
		for _, t := range inWindow[1:] {
			if t.Before(oldest) {
				oldest = t
			}
		}
		if left := oldest.Add(WhatsAppWindow).Sub(now); left > 0 {
			resetIn = int(left.Seconds()) + 1
		}
	}

	return RateStatus{
		SentInWindow: len(inWindow),
		WindowCap:    WhatsAppWindowCap,
		WindowHours:  int(WhatsAppWindow.Hours()),
		CooldownLeft: 0, // no per-message cooldown — only the window cap applies
		ResetIn:      resetIn,
		CapReached:   capReached,
		Blocked:      capReached,
	}, nil
}

// MarkSent records a contact as reached out to (idempotent by key+channel;
// prepended so newest is first). Returns the full sent list.
func MarkSent(path string, rec SentRecord) ([]SentRecord, error) {
	sentMu.Lock()
	defer sentMu.Unlock()

	recs, err := loadSentLocked(path)
	if err != nil {
		return nil, err
	}
	rec.Key = strings.TrimSpace(strings.ToLower(rec.Key))
	if rec.Key == "" {
		return recs, nil // nothing to key on
	}
	// De-dupe by key+channel: if already present, leave the existing (older) time.
	for _, r := range recs {
		if r.Key == rec.Key && r.Channel == rec.Channel {
			return recs, nil
		}
	}
	if rec.SentAt.IsZero() {
		// Caller should pass a time; fall back defensively.
		rec.SentAt = time.Now()
	}
	recs = append([]SentRecord{rec}, recs...)

	b, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, err
	}
	return recs, nil
}
