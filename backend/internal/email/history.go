package email

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// HistoryEntry records one send attempt.
type HistoryEntry struct {
	RecipientEmail string    `json:"recipientEmail"`
	Company        string    `json:"company"`
	Subject        string    `json:"subject"`
	Status         string    `json:"status"` // "sent" or "failed"
	Error          string    `json:"error,omitempty"`
	SentAt         time.Time `json:"sentAt"`
}

// historyMu guards concurrent read/modify/write of the history file.
var historyMu sync.Mutex

// LoadHistory reads all history entries, newest first. Missing file → empty slice.
func LoadHistory(path string) ([]HistoryEntry, error) {
	historyMu.Lock()
	defer historyMu.Unlock()
	return loadHistoryLocked(path)
}

func loadHistoryLocked(path string) ([]HistoryEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryEntry{}, nil
		}
		return nil, err
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// AppendHistory adds an entry (prepended so newest is first) and persists.
func AppendHistory(path string, entry HistoryEntry) error {
	historyMu.Lock()
	defer historyMu.Unlock()

	entries, err := loadHistoryLocked(path)
	if err != nil {
		return err
	}
	entries = append([]HistoryEntry{entry}, entries...)

	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
