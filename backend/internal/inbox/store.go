package inbox

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

// Reply statuses.
const (
	StatusNeedsReply = "needs_reply"
	StatusScheduled  = "scheduled"
	StatusNoAction   = "no_action"
	StatusReplied    = "replied"
	StatusDismissed  = "dismissed"
)

// ReplyRecord is one triaged inbox reply, persisted to replies.json.
type ReplyRecord struct {
	MessageID  string     `json:"messageId"`
	FromEmail  string     `json:"fromEmail"`
	FromName   string     `json:"fromName"`
	Subject    string     `json:"subject"`
	Body       string     `json:"body"`
	ReceivedAt time.Time  `json:"receivedAt"`
	Category   string     `json:"category"`
	Draft      string     `json:"draft"`
	Reason     string     `json:"reason"`
	Status     string     `json:"status"`
	FollowUpAt *time.Time `json:"followUpAt,omitempty"`
	// InReplyTo / references let a sent reply thread correctly.
	InReplyTo  string   `json:"inReplyTo,omitempty"`
	References []string `json:"references,omitempty"`
}

var repliesMu sync.Mutex

func loadRepliesLocked(path string) ([]ReplyRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ReplyRecord{}, nil
		}
		return nil, err
	}
	var recs []ReplyRecord
	if err := json.Unmarshal(b, &recs); err != nil {
		return nil, err
	}
	if recs == nil {
		recs = []ReplyRecord{}
	}
	return recs, nil
}

func saveRepliesLocked(path string, recs []ReplyRecord) error {
	b, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadReplies returns all stored replies, newest first.
func LoadReplies(path string) ([]ReplyRecord, error) {
	repliesMu.Lock()
	defer repliesMu.Unlock()
	recs, err := loadRepliesLocked(path)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].ReceivedAt.After(recs[j].ReceivedAt) })
	return recs, nil
}

// Merge adds freshly-triaged replies, de-duped by MessageID. Existing records
// keep their status (so a user's "replied"/"dismissed" or edits aren't undone by
// a re-check); genuinely new replies are added. Returns the full stored list.
func Merge(path string, incoming []ReplyRecord) ([]ReplyRecord, error) {
	repliesMu.Lock()
	defer repliesMu.Unlock()

	existing, err := loadRepliesLocked(path)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]bool, len(existing))
	for _, r := range existing {
		if r.MessageID != "" {
			byID[r.MessageID] = true
		}
	}
	for _, r := range incoming {
		// Fall back to sender+subject as a key when MessageID is missing.
		key := r.MessageID
		if key == "" {
			key = r.FromEmail + "|" + r.Subject
		}
		if byID[key] {
			continue
		}
		byID[key] = true
		existing = append(existing, r)
	}
	if err := saveRepliesLocked(path, existing); err != nil {
		return nil, err
	}
	sort.SliceStable(existing, func(i, j int) bool { return existing[i].ReceivedAt.After(existing[j].ReceivedAt) })
	return existing, nil
}

// SetStatus updates one reply's status (e.g. replied, dismissed) by MessageID.
// Returns the updated list.
func SetStatus(path, messageID, status string) ([]ReplyRecord, error) {
	repliesMu.Lock()
	defer repliesMu.Unlock()

	recs, err := loadRepliesLocked(path)
	if err != nil {
		return nil, err
	}
	for i := range recs {
		if recs[i].MessageID == messageID {
			recs[i].Status = status
			break
		}
	}
	if err := saveRepliesLocked(path, recs); err != nil {
		return nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].ReceivedAt.After(recs[j].ReceivedAt) })
	return recs, nil
}

// Find returns the stored reply with the given MessageID, or ok=false.
func Find(path, messageID string) (ReplyRecord, bool, error) {
	repliesMu.Lock()
	defer repliesMu.Unlock()
	recs, err := loadRepliesLocked(path)
	if err != nil {
		return ReplyRecord{}, false, err
	}
	for _, r := range recs {
		if r.MessageID == messageID {
			return r, true, nil
		}
	}
	return ReplyRecord{}, false, nil
}

// StatusForVerdict maps a triage verdict to the initial record status.
func StatusForVerdict(v Verdict) string {
	switch v.Category {
	case CatNotOpen:
		return StatusNoAction
	case CatFollowUpLater:
		return StatusScheduled
	default:
		return StatusNeedsReply
	}
}
