package batch

import (
	"encoding/json"
	"os"
	"time"
)

// persisted is the on-disk shape of the queue. The queue outlives the process
// (a window may be days away), so everything needed to resume is written here.
type persisted struct {
	Items    []Item    `json:"items"`
	Track    string    `json:"track"`
	Paused   bool      `json:"paused"`
	SendNow  bool      `json:"sendNow,omitempty"`
	QueuedAt time.Time `json:"queuedAt,omitempty"`
	// WindowStart identifies the window the SentInWindow counter belongs to.
	// When the current window differs, the counter resets.
	WindowStart  time.Time `json:"windowStart,omitempty"`
	SentInWindow int       `json:"sentInWindow"`
	LastSentAt   time.Time `json:"lastSentAt,omitempty"`
}

// load reads the persisted queue. A missing file is an empty queue.
func load(path string) (persisted, error) {
	var p persisted
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return persisted{}, err
	}
	return p, nil
}

// save writes the queue atomically-ish (write temp, rename) so a crash mid-write
// can't leave a truncated queue file behind.
func save(path string, p persisted) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
