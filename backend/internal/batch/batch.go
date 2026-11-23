// Package batch runs a persisted, schedule-gated queue of outgoing emails.
//
// Recipients can be added at any time, but they only go out inside the weekly
// send windows configured in internal/schedule (e.g. Mon & Wed 11:00–13:00) —
// cold outreach read during working hours gets seen, outreach that lands at
// 2am does not. Anything queued outside a window waits for the next one.
//
// Within a window the queue drains one email at a time with a random gap
// between sends (to stay under Gmail's radar), up to the schedule's per-window
// cap. Leftovers roll over to the following window.
//
// The queue is written to disk on every state change, so a backend restart
// between windows loses nothing.
package batch

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"emailsender/internal/schedule"
)

// Status is the state of one queued email.
type Status string

const (
	StatusQueued  Status = "queued"
	StatusSending Status = "sending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped" // cleared before it ran
)

// Item is one recipient in the queue.
type Item struct {
	Email   string     `json:"email"`
	Company string     `json:"company"`
	Name    string     `json:"name"`
	Status  Status     `json:"status"`
	Error   string     `json:"error,omitempty"`
	SentAt  *time.Time `json:"sentAt,omitempty"`
}

// Snapshot is a read-only view of the queue for the UI.
type Snapshot struct {
	Track     string `json:"track"`
	Total     int    `json:"total"`
	Sent      int    `json:"sent"`
	Failed    int    `json:"failed"`
	Remaining int    `json:"remaining"` // still queued (waiting for a window or a turn)
	Paused    bool   `json:"paused"`
	SendNow   bool   `json:"sendNow"` // window override active: draining regardless of schedule
	Items     []Item `json:"items"`

	// Scheduling state.
	WindowOpen   bool   `json:"windowOpen"`   // a send window is open right now
	Sending      bool   `json:"sending"`      // actively draining (window open, work left, not paused)
	NextInSec    int    `json:"nextInSec"`    // seconds until the next send fires (this window)
	NextWindowAt string `json:"nextWindowAt"` // RFC3339 start of the next window (empty if none)
	OpensInSec   int    `json:"opensInSec"`   // seconds until the next window opens
	WindowEndsAt string `json:"windowEndsAt"` // RFC3339 end of the open window
	SentInWindow int    `json:"sentInWindow"` // sends used in the current window
	PerWindowCap int    `json:"perWindowCap"` // 0 = uncapped
	CapReached   bool   `json:"capReached"`   // window cap hit; rest rolls to the next window
	QueuedAt     string `json:"queuedAt,omitempty"`
	LastSentAt   string `json:"lastSentAt,omitempty"`
}

// SendFunc processes one item: compose (AI) + send + record history. Returning
// an error marks the item failed. The manager handles pacing and status.
type SendFunc func(ctx context.Context, track string, it Item) error

// Manager owns the single persisted queue and the worker that drains it.
type Manager struct {
	mu   sync.Mutex
	cond *sync.Cond // wakes the worker on add/resume/schedule change/shutdown

	path  string
	sched *schedule.Store
	send  SendFunc

	items  []Item
	track  string
	paused bool
	// sendNow overrides the schedule: drain the queue immediately regardless of
	// the send windows. Cleared once the queue runs dry.
	sendNow bool

	queuedAt   time.Time
	lastSentAt time.Time

	// Per-window cap accounting: how many sends have happened inside the window
	// that started at windowStart.
	windowStart  time.Time
	sentInWindow int

	nextAt    time.Time // when the next send in this window is due
	maxGapSec int
	stopped   bool
	done      chan struct{} // closed when the worker loop has exited
}

// NewManager loads any persisted queue from path and returns a manager ready to
// Start. A corrupt queue file is logged and treated as empty rather than
// blocking startup.
func NewManager(path string, sched *schedule.Store, maxGapSec int) *Manager {
	m := &Manager{path: path, sched: sched, maxGapSec: maxGapSec}
	m.cond = sync.NewCond(&m.mu)

	p, err := load(path)
	if err != nil {
		log.Printf("warning: could not read the send queue (%v) — starting empty", err)
	} else {
		m.items = p.Items
		m.track = p.Track
		m.paused = p.Paused
		m.sendNow = p.SendNow
		m.queuedAt = p.QueuedAt
		m.windowStart = p.WindowStart
		m.sentInWindow = p.SentInWindow
		m.lastSentAt = p.LastSentAt
		// Anything caught mid-send by a restart goes back to queued.
		for i := range m.items {
			if m.items[i].Status == StatusSending {
				m.items[i].Status = StatusQueued
			}
		}
	}
	return m
}

// Start launches the background worker. It runs for the life of the process,
// sleeping until a window opens and work exists.
func (m *Manager) Start(send SendFunc) {
	m.mu.Lock()
	m.send = send
	m.done = make(chan struct{})
	m.mu.Unlock()
	go m.run()
	go m.tick()
}

// tick nudges the worker every 30s so it re-evaluates the clock — windows open
// on wall-time, not on an event we can wait for.
func (m *Manager) tick() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		m.mu.Lock()
		stopped := m.stopped
		m.mu.Unlock()
		if stopped {
			return
		}
		m.cond.Broadcast()
	}
}

// Stop shuts the worker down and waits for it to exit, so no further writes to
// the queue file happen after Stop returns. Safe to call more than once.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	done := m.done
	m.mu.Unlock()
	m.cond.Broadcast()

	if done == nil {
		return // never Start()ed
	}
	// The worker may be mid-send or mid-gap; don't hang shutdown on it forever.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// nextQueuedLocked returns the index of the next item to send, or -1.
func (m *Manager) nextQueuedLocked() int {
	for i := range m.items {
		if m.items[i].Status == StatusQueued {
			return i
		}
	}
	return -1
}

// indexOfSendingLocked finds the in-flight item for an email, or -1 if it was
// cleared while the send was running.
func (m *Manager) indexOfSendingLocked(email string) int {
	for i := range m.items {
		if m.items[i].Status == StatusSending && lower(m.items[i].Email) == lower(email) {
			return i
		}
	}
	return -1
}

// rollWindowLocked resets the per-window counter when the current window is not
// the one the counter was recorded against.
func (m *Manager) rollWindowLocked(now time.Time) {
	s := m.sched.Get()
	start, open := s.CurrentWindowStart(now)
	if !open {
		return
	}
	if !start.Equal(m.windowStart) {
		m.windowStart = start
		m.sentInWindow = 0
	}
}

// canSendNowLocked reports whether a send may fire at `now`: a window must be
// open, the queue unpaused, the per-window cap not yet reached, and work left.
func (m *Manager) canSendNowLocked(now time.Time) bool {
	if m.paused || m.send == nil {
		return false
	}
	if m.nextQueuedLocked() < 0 {
		return false
	}
	// "Send now" bypasses both the window and the per-window cap — it's an
	// explicit, deliberate override for urgent sends.
	if m.sendNow {
		return true
	}
	s := m.sched.Get()
	if !s.Open(now) {
		return false
	}
	m.rollWindowLocked(now)
	if s.PerWindowCap > 0 && m.sentInWindow >= s.PerWindowCap {
		return false
	}
	return true
}

// run is the worker loop: wait until a send is allowed, send one, pace, repeat.
func (m *Manager) run() {
	defer func() {
		m.mu.Lock()
		if m.done != nil {
			close(m.done)
			m.done = nil
		}
		m.mu.Unlock()
	}()

	for {
		m.mu.Lock()
		// Wait until there is something we're allowed to send. cond is broadcast
		// by Add/Resume/schedule changes and by the 30s ticker (for the clock).
		for !m.stopped && !m.canSendNowLocked(time.Now()) {
			m.nextAt = time.Time{}
			// The override is a one-shot for the work that was pending: once
			// nothing is left to send, revert to the normal schedule so a later
			// paste doesn't silently bypass the windows.
			if m.sendNow && m.nextQueuedLocked() < 0 {
				m.sendNow = false
				m.persistLocked()
			}
			m.cond.Wait()
		}
		if m.stopped {
			m.mu.Unlock()
			return
		}

		idx := m.nextQueuedLocked()
		m.items[idx].Status = StatusSending
		item := m.items[idx]
		track := m.track
		send := m.send
		m.persistLocked()
		m.mu.Unlock()

		err := send(context.Background(), track, item)

		m.mu.Lock()
		now := time.Now()
		// Re-find the item by email: Clear/ClearAll may have reindexed items
		// while the send was in flight, so the old index can't be trusted.
		if i := m.indexOfSendingLocked(item.Email); i >= 0 {
			if err != nil {
				m.items[i].Status = StatusFailed
				m.items[i].Error = err.Error()
			} else {
				m.items[i].Status = StatusSent
				m.items[i].Error = ""
				sentAt := now
				m.items[i].SentAt = &sentAt
				m.lastSentAt = now
			}
		} else if err == nil {
			m.lastSentAt = now
		}
		// Only a delivered email consumes a slot. The cap exists to protect the
		// Gmail account's sending reputation, and a send that never reached Gmail
		// (network timeout) didn't touch it — counting those would silently eat
		// the window's budget on a flaky connection.
		m.rollWindowLocked(now)
		if err == nil {
			m.sentInWindow++
		}

		// Pace the next send with a random gap, but never past the window's end.
		gap := time.Duration(rand.Intn(m.maxGapSec+1)) * time.Second
		m.nextAt = now.Add(gap)
		m.persistLocked()
		stopped := m.stopped
		m.mu.Unlock()

		if stopped {
			return
		}
		time.Sleep(gap)
	}
}

// Add appends recipients to the queue, skipping emails already queued or
// already sent in this queue. Returns how many were actually added.
func (m *Manager) Add(items []Item, track string) int {
	m.mu.Lock()

	existing := make(map[string]bool, len(m.items))
	for _, it := range m.items {
		existing[lower(it.Email)] = true
	}

	added := 0
	for _, it := range items {
		if existing[lower(it.Email)] {
			continue
		}
		existing[lower(it.Email)] = true
		it.Status = StatusQueued
		it.Error = ""
		it.SentAt = nil
		m.items = append(m.items, it)
		added++
	}
	if added > 0 {
		m.track = track
		if m.queuedAt.IsZero() {
			m.queuedAt = time.Now()
		}
		m.persistLocked()
	}
	m.mu.Unlock()

	m.cond.Broadcast() // a new item may be sendable right now
	return added
}

// Pause holds the queue. An in-flight send finishes; nothing new starts.
func (m *Manager) Pause() {
	m.mu.Lock()
	m.paused = true
	m.persistLocked()
	m.mu.Unlock()
}

// Resume un-pauses the queue.
func (m *Manager) Resume() {
	m.mu.Lock()
	m.paused = false
	m.persistLocked()
	m.mu.Unlock()
	m.cond.Broadcast()
}

// Clear removes every item still waiting, marking them skipped and then
// dropping them. Sent/failed history stays until ClearAll.
func (m *Manager) Clear() {
	m.mu.Lock()
	kept := m.items[:0]
	for _, it := range m.items {
		if it.Status == StatusQueued {
			continue // drop it
		}
		kept = append(kept, it)
	}
	m.items = append([]Item(nil), kept...)
	m.persistLocked()
	m.mu.Unlock()
}

// ClearAll empties the queue entirely, including finished items.
func (m *Manager) ClearAll() {
	m.mu.Lock()
	// Keep anything mid-flight so the worker's index stays valid.
	kept := make([]Item, 0, len(m.items))
	for _, it := range m.items {
		if it.Status == StatusSending {
			kept = append(kept, it)
		}
	}
	m.items = kept
	m.queuedAt = time.Time{}
	m.persistLocked()
	m.mu.Unlock()
}

// Notify wakes the worker — called when the schedule changes so a newly-opened
// window starts draining without waiting for the next tick.
func (m *Manager) Notify() { m.cond.Broadcast() }

// RetryFailed puts every failed item back in the queue so it goes out in the
// next window. Useful after a network blip: the recipients were never actually
// emailed, so re-queuing them doesn't risk a duplicate. Returns how many.
func (m *Manager) RetryFailed() int {
	m.mu.Lock()
	n := 0
	for i := range m.items {
		if m.items[i].Status == StatusFailed {
			m.items[i].Status = StatusQueued
			m.items[i].Error = ""
			n++
		}
	}
	if n > 0 {
		m.persistLocked()
	}
	m.mu.Unlock()

	m.cond.Broadcast()
	return n
}

// SendNow drains the queue immediately, ignoring the send windows, and returns
// how many items it will attempt. The override lasts until the queue empties
// (or is turned off), so a queue added outside a window can go out at once.
func (m *Manager) SendNow() int {
	m.mu.Lock()
	m.sendNow = true
	m.paused = false
	n := 0
	for _, it := range m.items {
		if it.Status == StatusQueued {
			n++
		}
	}
	m.persistLocked()
	m.mu.Unlock()

	m.cond.Broadcast()
	return n
}

// CancelSendNow turns the window override back off, returning the queue to the
// normal schedule.
func (m *Manager) CancelSendNow() {
	m.mu.Lock()
	m.sendNow = false
	m.persistLocked()
	m.mu.Unlock()
}

// Get builds a snapshot for the UI.
func (m *Manager) Get() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	s := m.sched.Get()

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

	// Window accounting for display. Mirror rollWindowLocked without mutating:
	// if the counter belongs to an older window it reads as 0.
	windowOpen := s.Open(now)
	sentInWindow := m.sentInWindow
	if start, open := s.CurrentWindowStart(now); open && !start.Equal(m.windowStart) {
		sentInWindow = 0
	} else if !open && s.Enabled {
		sentInWindow = 0
	}

	capReached := s.PerWindowCap > 0 && sentInWindow >= s.PerWindowCap && windowOpen
	// The override ignores both the window and the cap, so it's "sending" as
	// long as work remains and it isn't paused.
	sending := (windowOpen && !capReached || m.sendNow) && !m.paused && remaining > 0

	snap := Snapshot{
		Track:        m.track,
		Total:        len(m.items),
		Sent:         sent,
		Failed:       failed,
		Remaining:    remaining,
		Paused:       m.paused,
		SendNow:      m.sendNow,
		Items:        append([]Item(nil), m.items...),
		WindowOpen:   windowOpen,
		Sending:      sending,
		SentInWindow: sentInWindow,
		PerWindowCap: s.PerWindowCap,
		CapReached:   capReached,
	}

	if sending && !m.nextAt.IsZero() {
		if d := time.Until(m.nextAt); d > 0 {
			snap.NextInSec = int(d.Seconds()) + 1
		}
	}
	if end, open := s.CurrentWindowEnd(now); open {
		snap.WindowEndsAt = end.Format(time.RFC3339)
	}
	// The next window matters both when closed (when will it start?) and when
	// open but capped (when does the rest go out?).
	if next, ok := s.NextOpen(now); ok && (!windowOpen || capReached) {
		if !windowOpen {
			snap.NextWindowAt = next.Format(time.RFC3339)
			if d := time.Until(next); d > 0 {
				snap.OpensInSec = int(d.Seconds()) + 1
			}
		} else if end, open := s.CurrentWindowEnd(now); open {
			// Capped inside a window: the rest resumes at the window after this one.
			if after, ok := s.NextOpen(end); ok {
				snap.NextWindowAt = after.Format(time.RFC3339)
				if d := time.Until(after); d > 0 {
					snap.OpensInSec = int(d.Seconds()) + 1
				}
			}
		}
	}
	if !m.queuedAt.IsZero() {
		snap.QueuedAt = m.queuedAt.Format(time.RFC3339)
	}
	if !m.lastSentAt.IsZero() {
		snap.LastSentAt = m.lastSentAt.Format(time.RFC3339)
	}
	return snap
}

// persistLocked writes the queue to disk. Caller must hold m.mu.
func (m *Manager) persistLocked() {
	err := save(m.path, persisted{
		Items:        m.items,
		Track:        m.track,
		Paused:       m.paused,
		SendNow:      m.sendNow,
		QueuedAt:     m.queuedAt,
		WindowStart:  m.windowStart,
		SentInWindow: m.sentInWindow,
		LastSentAt:   m.lastSentAt,
	})
	if err != nil {
		log.Printf("warning: could not save the send queue: %v", err)
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
