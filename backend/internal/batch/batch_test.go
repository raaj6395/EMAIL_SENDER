package batch

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"emailsender/internal/schedule"
)

// newTestManager builds a manager over a temp queue file with the given
// schedule, and a send func that records every recipient it was asked to send.
func newTestManager(t *testing.T, s schedule.Schedule) (*Manager, *[]string, *sync.Mutex) {
	t.Helper()
	dir := t.TempDir()

	schedStore, err := schedule.LoadStore(filepath.Join(dir, "schedule.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := schedStore.Set(s); err != nil {
		t.Fatalf("Set schedule: %v", err)
	}

	var mu sync.Mutex
	var sent []string
	m := NewManager(filepath.Join(dir, "queue.json"), schedStore, 0)
	m.Start(func(ctx context.Context, track string, it Item) error {
		mu.Lock()
		sent = append(sent, it.Email)
		mu.Unlock()
		return nil
	})
	t.Cleanup(m.Stop)
	return m, &sent, &mu
}

// alwaysClosed is a schedule whose only window is on a day/time we're not in.
// It uses every weekday except today's, so no window can be open during the test.
func alwaysClosed() schedule.Schedule {
	today := time.Now().Weekday()
	var ws []schedule.Window
	for d := time.Sunday; d <= time.Saturday; d++ {
		if d == today {
			continue
		}
		ws = append(ws, schedule.Window{Day: d, StartMin: 11 * 60, EndMin: 13 * 60})
	}
	return schedule.Schedule{Enabled: true, Windows: ws, PerWindowCap: 0}
}

// alwaysOpen is a schedule with a full-day window every day.
func alwaysOpen(cap int) schedule.Schedule {
	var ws []schedule.Window
	for d := time.Sunday; d <= time.Saturday; d++ {
		ws = append(ws, schedule.Window{Day: d, StartMin: 0, EndMin: 24 * 60})
	}
	return schedule.Schedule{Enabled: true, Windows: ws, PerWindowCap: cap}
}

func waitFor(t *testing.T, cond func() bool, d time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", msg)
}

func TestQueueHoldsOutsideWindow(t *testing.T) {
	m, sent, mu := newTestManager(t, alwaysClosed())

	added := m.Add([]Item{
		{Email: "a@stripe.com", Company: "Stripe"},
		{Email: "b@figma.com", Company: "Figma"},
	}, "sd")
	if added != 2 {
		t.Fatalf("Add = %d, want 2", added)
	}

	// Give the worker a chance to (incorrectly) fire.
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	n := len(*sent)
	mu.Unlock()
	if n != 0 {
		t.Errorf("sent %d emails outside the send window, want 0", n)
	}

	snap := m.Get()
	if snap.WindowOpen {
		t.Error("WindowOpen = true outside any window")
	}
	if snap.Remaining != 2 {
		t.Errorf("Remaining = %d, want 2", snap.Remaining)
	}
	if snap.NextWindowAt == "" {
		t.Error("NextWindowAt should tell the user when the queue drains")
	}
}

func TestQueueDrainsInsideWindow(t *testing.T) {
	m, sent, mu := newTestManager(t, alwaysOpen(0))

	m.Add([]Item{
		{Email: "a@stripe.com", Company: "Stripe"},
		{Email: "b@figma.com", Company: "Figma"},
		{Email: "c@linear.app", Company: "Linear"},
	}, "sd")

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(*sent) == 3
	}, 3*time.Second, "all three emails to send")

	snap := m.Get()
	if snap.Sent != 3 || snap.Remaining != 0 {
		t.Errorf("Sent=%d Remaining=%d, want 3 and 0", snap.Sent, snap.Remaining)
	}
}

func TestPerWindowCapStopsSending(t *testing.T) {
	m, sent, mu := newTestManager(t, alwaysOpen(2))

	m.Add([]Item{
		{Email: "a@stripe.com"},
		{Email: "b@figma.com"},
		{Email: "c@linear.app"},
		{Email: "d@notion.so"},
	}, "sd")

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(*sent) == 2
	}, 3*time.Second, "the cap to be reached")

	// Past the cap, nothing more may go out in this window.
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	n := len(*sent)
	mu.Unlock()
	if n != 2 {
		t.Errorf("sent %d emails, want exactly the cap of 2", n)
	}

	snap := m.Get()
	if !snap.CapReached {
		t.Error("CapReached should be true once the window cap is hit")
	}
	if snap.Remaining != 2 {
		t.Errorf("Remaining = %d, want 2 rolled to the next window", snap.Remaining)
	}
}

func TestPauseBlocksSendingInsideWindow(t *testing.T) {
	m, sent, mu := newTestManager(t, alwaysOpen(0))
	m.Pause()

	m.Add([]Item{{Email: "a@stripe.com"}}, "sd")
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	n := len(*sent)
	mu.Unlock()
	if n != 0 {
		t.Errorf("sent %d while paused, want 0", n)
	}

	m.Resume()
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(*sent) == 1
	}, 3*time.Second, "the send to resume")
}

// newFailingManager is like newTestManager but its send func always fails,
// which is what a network timeout looks like to the queue.
func newFailingManager(t *testing.T, s schedule.Schedule) (*Manager, *int32) {
	t.Helper()
	dir := t.TempDir()
	schedStore, err := schedule.LoadStore(filepath.Join(dir, "schedule.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := schedStore.Set(s); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var attempts int32
	m := NewManager(filepath.Join(dir, "queue.json"), schedStore, 0)
	m.Start(func(ctx context.Context, track string, it Item) error {
		atomic.AddInt32(&attempts, 1)
		return errTest
	})
	t.Cleanup(m.Stop)
	return m, &attempts
}

var errTest = &testError{"network timeout"}

type testError struct{ s string }

func (e *testError) Error() string { return e.s }

func TestSendNowBypassesClosedWindow(t *testing.T) {
	m, sent, mu := newTestManager(t, alwaysClosed())

	m.Add([]Item{{Email: "a@stripe.com"}, {Email: "b@figma.com"}}, "sd")

	// Confirm the window really is shut.
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	n := len(*sent)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("sent %d before Send Now, want 0", n)
	}

	if got := m.SendNow(); got != 2 {
		t.Errorf("SendNow reported %d pending, want 2", got)
	}

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(*sent) == 2
	}, 3*time.Second, "Send Now to drain the queue outside a window")

	// Once drained, the override releases so later adds obey the schedule again.
	waitFor(t, func() bool { return !m.Get().SendNow }, 2*time.Second, "the override to clear")
}

func TestSendNowIgnoresWindowCap(t *testing.T) {
	m, sent, mu := newTestManager(t, alwaysOpen(1)) // cap of 1

	m.Add([]Item{{Email: "a@x.com"}, {Email: "b@x.com"}, {Email: "c@x.com"}}, "sd")
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(*sent) == 1
	}, 3*time.Second, "the cap to stop sending")

	m.SendNow()
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(*sent) == 3
	}, 3*time.Second, "Send Now to push past the cap")
}

func TestFailedSendsDoNotConsumeWindowCap(t *testing.T) {
	// Cap of 2, but every send fails: the cap must not be eaten, so all three
	// items get attempted rather than stopping after two failures.
	m, attempts := newFailingManager(t, alwaysOpen(2))
	m.Add([]Item{{Email: "a@x.com"}, {Email: "b@x.com"}, {Email: "c@x.com"}}, "sd")

	waitFor(t, func() bool { return atomic.LoadInt32(attempts) == 3 }, 3*time.Second,
		"all items to be attempted despite the cap")

	snap := m.Get()
	if snap.Failed != 3 {
		t.Errorf("Failed = %d, want 3", snap.Failed)
	}
	if snap.SentInWindow != 0 {
		t.Errorf("SentInWindow = %d, want 0 — failures must not consume the cap", snap.SentInWindow)
	}
}

func TestRetryFailedRequeues(t *testing.T) {
	m, attempts := newFailingManager(t, alwaysOpen(0))
	m.Add([]Item{{Email: "a@x.com"}, {Email: "b@x.com"}}, "sd")

	waitFor(t, func() bool { return m.Get().Failed == 2 }, 3*time.Second, "both sends to fail")
	first := atomic.LoadInt32(attempts)

	if got := m.RetryFailed(); got != 2 {
		t.Errorf("RetryFailed = %d, want 2", got)
	}

	// They should be attempted again (and fail again, since the sender always does).
	waitFor(t, func() bool { return atomic.LoadInt32(attempts) >= first+2 }, 3*time.Second,
		"re-queued items to be retried")

	snap := m.Get()
	if snap.Sent != 0 {
		t.Errorf("Sent = %d, want 0", snap.Sent)
	}
}

func TestAddSkipsDuplicates(t *testing.T) {
	m, _, _ := newTestManager(t, alwaysClosed())

	if got := m.Add([]Item{{Email: "a@stripe.com"}, {Email: "A@Stripe.com"}}, "sd"); got != 1 {
		t.Errorf("Add with a case-variant duplicate = %d, want 1", got)
	}
	if got := m.Add([]Item{{Email: "a@stripe.com"}}, "sd"); got != 0 {
		t.Errorf("re-adding an existing recipient = %d, want 0", got)
	}
}

func TestQueuePersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	queuePath := filepath.Join(dir, "queue.json")
	schedStore, err := schedule.LoadStore(filepath.Join(dir, "schedule.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := schedStore.Set(alwaysClosed()); err != nil {
		t.Fatalf("Set: %v", err)
	}

	m1 := NewManager(queuePath, schedStore, 0)
	m1.Add([]Item{{Email: "a@stripe.com", Company: "Stripe"}}, "ai")

	// A fresh manager over the same file sees the queued work.
	m2 := NewManager(queuePath, schedStore, 0)
	snap := m2.Get()
	if snap.Remaining != 1 {
		t.Errorf("Remaining after restart = %d, want 1", snap.Remaining)
	}
	if snap.Track != "ai" {
		t.Errorf("Track after restart = %q, want \"ai\"", snap.Track)
	}
	if len(snap.Items) != 1 || snap.Items[0].Email != "a@stripe.com" {
		t.Errorf("items did not survive the restart: %+v", snap.Items)
	}
}

func TestClearDropsWaitingItems(t *testing.T) {
	m, _, _ := newTestManager(t, alwaysClosed())
	m.Add([]Item{{Email: "a@stripe.com"}, {Email: "b@figma.com"}}, "sd")

	m.Clear()
	if snap := m.Get(); snap.Remaining != 0 {
		t.Errorf("Remaining after Clear = %d, want 0", snap.Remaining)
	}
}

func TestConcurrentAddAndSnapshot(t *testing.T) {
	m, _, _ := newTestManager(t, alwaysOpen(0))

	var wg sync.WaitGroup
	var reads int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Add([]Item{{Email: string(rune('a'+i)) + "@x.com"}}, "sd")
		}(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Get()
			atomic.AddInt64(&reads, 1)
		}()
	}
	wg.Wait() // -race makes this meaningful
	if reads == 0 {
		t.Error("expected snapshot reads")
	}
}
