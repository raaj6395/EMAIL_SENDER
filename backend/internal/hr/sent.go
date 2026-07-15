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

// WhatsApp send-rate limits, chosen from research on avoiding bans for cold
// outreach from a personal number (see the app docs / commit message):
//   • a randomized 30-60s gap between sends — we enforce a 30s hard floor and
//     let the UI display a longer suggested wait;
//   • a conservative daily cap for a new/unwarmed number.
const (
	WhatsAppCooldown = 30 * time.Second
	WhatsAppDailyCap = 15
)

// RateStatus describes whether a new send is currently allowed for a channel.
type RateStatus struct {
	SentToday      int   `json:"sentToday"`
	DailyCap       int   `json:"dailyCap"`
	CooldownLeft   int   `json:"cooldownLeft"`   // seconds until the cooldown clears
	Blocked        bool  `json:"blocked"`        // true if a send is not allowed right now
	CapReached     bool  `json:"capReached"`     // true if the daily cap is hit
}

// WhatsAppRateStatus computes the current WhatsApp send-rate status from the
// recorded sent timestamps: how many were sent today (local time) and how long
// until the inter-send cooldown clears.
func WhatsAppRateStatus(path string) (RateStatus, error) {
	sentMu.Lock()
	defer sentMu.Unlock()
	recs, err := loadSentLocked(path)
	if err != nil {
		return RateStatus{}, err
	}

	now := time.Now()
	y, m, d := now.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location())

	sentToday := 0
	var last time.Time
	for _, r := range recs {
		if r.Channel != "whatsapp" {
			continue
		}
		if r.SentAt.After(dayStart) {
			sentToday++
		}
		if r.SentAt.After(last) {
			last = r.SentAt
		}
	}

	cooldownLeft := 0
	if !last.IsZero() {
		if left := WhatsAppCooldown - now.Sub(last); left > 0 {
			cooldownLeft = int(left.Seconds()) + 1
		}
	}
	capReached := sentToday >= WhatsAppDailyCap

	return RateStatus{
		SentToday:    sentToday,
		DailyCap:     WhatsAppDailyCap,
		CooldownLeft: cooldownLeft,
		CapReached:   capReached,
		Blocked:      capReached || cooldownLeft > 0,
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
