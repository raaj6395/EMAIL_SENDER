// Package batch runs a paced queue of outgoing emails in the background so the
// user can bulk-send without doing it one-by-one. It sends one email at a time
// with a random gap between sends (to stay under Gmail's radar and protect
// deliverability) and exposes a progress snapshot the UI can poll. Only one
// batch runs at a time.
package batch

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Status is the state of one queued email.
type Status string

const (
	StatusQueued  Status = "queued"
	StatusSending Status = "sending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped" // e.g. cancelled before it ran
)

// Item is one recipient in the batch.
type Item struct {
	Email   string `json:"email"`
	Company string `json:"company"`
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Error   string `json:"error,omitempty"`
}

// Snapshot is a read-only view of the batch for the UI.
type Snapshot struct {
	Active    bool   `json:"active"`    // a batch is currently running
	Track     string `json:"track"`     // "sd" | "ai"
	Total     int    `json:"total"`
	Sent      int    `json:"sent"`
	Failed    int    `json:"failed"`
	Remaining int    `json:"remaining"`
	NextInSec int    `json:"nextInSec"` // approx seconds until the next send fires
	Items     []Item `json:"items"`
	StartedAt string `json:"startedAt,omitempty"`
	Done      bool   `json:"done"` // finished (or cancelled) — items reflect final state
}

// SendFunc processes one item: compose (AI) + send + record history. It returns
// an error if the send failed. The manager handles pacing and status.
type SendFunc func(ctx context.Context, track string, it Item) error

// Manager runs at most one batch at a time.
type Manager struct {
	mu        sync.Mutex
	items     []Item
	track     string
	active    bool
	done      bool
	startedAt time.Time
	nextAt    time.Time // when the next send is scheduled to fire
	cancel    context.CancelFunc
}

// NewManager creates an idle batch manager.
func NewManager() *Manager { return &Manager{} }

// Running reports whether a batch is currently in progress.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// Start begins processing items in the background. It returns false if a batch
// is already running (only one at a time). maxGapSec is the upper bound of the
// random inter-send gap (0..maxGapSec seconds).
func (m *Manager) Start(items []Item, track string, maxGapSec int, send SendFunc) bool {
	m.mu.Lock()
	if m.active {
		m.mu.Unlock()
		return false
	}
	// Reset state for the new batch.
	for i := range items {
		items[i].Status = StatusQueued
		items[i].Error = ""
	}
	m.items = items
	m.track = track
	m.active = true
	m.done = false
	m.startedAt = time.Now()
	m.nextAt = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.mu.Unlock()

	go m.run(ctx, maxGapSec, send)
	return true
}

// run processes each item sequentially with a random gap between sends.
func (m *Manager) run(ctx context.Context, maxGapSec int, send SendFunc) {
	defer func() {
		m.mu.Lock()
		m.active = false
		m.done = true
		m.cancel = nil
		m.mu.Unlock()
	}()

	for i := range m.items {
		if ctx.Err() != nil {
			m.markRemainingSkipped(i)
			return
		}

		m.setStatus(i, StatusSending, "")
		err := send(ctx, m.track, m.snapshotItem(i))
		if err != nil {
			m.setStatus(i, StatusFailed, err.Error())
		} else {
			m.setStatus(i, StatusSent, "")
		}

		// Random 0..maxGapSec gap before the next send (skip after the last one).
		if i < len(m.items)-1 {
			gap := time.Duration(rand.Intn(maxGapSec+1)) * time.Second
			m.mu.Lock()
			m.nextAt = time.Now().Add(gap)
			m.mu.Unlock()
			select {
			case <-time.After(gap):
			case <-ctx.Done():
				m.markRemainingSkipped(i + 1)
				return
			}
		}
	}
}

// Cancel stops the batch after the in-flight send; remaining items are skipped.
func (m *Manager) Cancel() {
	m.mu.Lock()
	c := m.cancel
	m.mu.Unlock()
	if c != nil {
		c()
	}
}

// Get returns a snapshot for the UI.
func (m *Manager) Get() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	sent, failed, remaining := 0, 0, 0
	for _, it := range m.items {
		switch it.Status {
		case StatusSent:
			sent++
		case StatusFailed:
			failed++
		case StatusQueued, StatusSending:
			remaining++
		}
	}
	nextIn := 0
	if m.active && !m.nextAt.IsZero() {
		if d := time.Until(m.nextAt); d > 0 {
			nextIn = int(d.Seconds()) + 1
		}
	}
	itemsCopy := make([]Item, len(m.items))
	copy(itemsCopy, m.items)

	snap := Snapshot{
		Active:    m.active,
		Track:     m.track,
		Total:     len(m.items),
		Sent:      sent,
		Failed:    failed,
		Remaining: remaining,
		NextInSec: nextIn,
		Items:     itemsCopy,
		Done:      m.done,
	}
	if !m.startedAt.IsZero() {
		snap.StartedAt = m.startedAt.Format(time.RFC3339)
	}
	return snap
}

func (m *Manager) setStatus(i int, s Status, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i >= 0 && i < len(m.items) {
		m.items[i].Status = s
		m.items[i].Error = errMsg
	}
}

func (m *Manager) snapshotItem(i int) Item {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[i]
}

func (m *Manager) markRemainingSkipped(from int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := from; i < len(m.items); i++ {
		if m.items[i].Status == StatusQueued || m.items[i].Status == StatusSending {
			m.items[i].Status = StatusSkipped
		}
	}
}
