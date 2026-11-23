package reach

import (
	"context"
	"sync"
	"time"

	"emailsender/internal/verify"
)

// verifyWorkers bounds concurrent MX lookups. These are DNS-bound, not CPU-
// bound, so a pool well above GOMAXPROCS is the point; 24 keeps a few thousand
// addresses to minutes without hammering the resolver.
const verifyWorkers = 24

// VerifyBatch checks up to `limit` not-yet-verified addresses from rows and
// merges the results into the cache. Already-cached addresses are skipped, so
// calling it repeatedly walks through the sheet. Returns how many were checked
// and how many of those are deliverable.
//
// Only the FIRST address of each row is checked until it fails: a row is
// reachable as soon as one of its addresses is good, so verifying the fallbacks
// of an already-reachable row is wasted work.
func VerifyBatch(ctx context.Context, path string, rows []Row, limit int) (checked, ok int, err error) {
	mu.Lock()
	c, err := loadLocked(path)
	if err != nil {
		mu.Unlock()
		return 0, 0, err
	}
	// Snapshot what's already known, then release the lock — the checks below
	// are slow network calls and must not block other readers.
	known := make(map[string]Verdict, len(c.Verdicts))
	for k, v := range c.Verdicts {
		known[k] = v
	}
	mu.Unlock()

	todo := pendingEmails(rows, known, limit)
	if len(todo) == 0 {
		return 0, 0, nil
	}

	results := make([]Verdict, len(todo))
	var wg sync.WaitGroup
	sem := make(chan struct{}, verifyWorkers)
	for i, email := range todo {
		wg.Add(1)
		go func(i int, email string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := verify.Check(ctx, email)
			results[i] = Verdict{
				Email:     email,
				Valid:     r.Valid,
				Code:      string(r.Code),
				Reason:    r.Reason,
				CheckedAt: time.Now().Format(time.RFC3339),
			}
		}(i, email)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	// Re-read: another request may have written while we were on the network.
	c, err = loadLocked(path)
	if err != nil {
		return 0, 0, err
	}
	for _, v := range results {
		// A transient DNS failure is not a real answer — don't cache it, or the
		// address is stuck with a guess forever. Retry it on the next run.
		if v.Code == string(verify.CodeError) {
			continue
		}
		c.Verdicts[v.Email] = v
		checked++
		if v.Valid {
			ok++
		}
	}
	if checked > 0 {
		if err := saveLocked(path, c); err != nil {
			return 0, 0, err
		}
	}
	return checked, ok, nil
}

// pendingEmails picks the next addresses needing a check, at most `limit`.
// Within a row, addresses are considered in order and the walk stops at the
// first one already known deliverable.
func pendingEmails(rows []Row, known map[string]Verdict, limit int) []string {
	seen := map[string]bool{}
	var todo []string
	for _, r := range rows {
		for _, e := range r.Emails {
			if v, ok := known[e]; ok {
				if v.Valid {
					break // this row already has a good address
				}
				continue // known bad — try the row's next address
			}
			if !seen[e] {
				seen[e] = true
				todo = append(todo, e)
				if limit > 0 && len(todo) >= limit {
					return todo
				}
			}
			break // one unchecked address per row per pass, so rows advance evenly
		}
	}
	return todo
}

// BestEmail returns the row's first address known to be deliverable, plus its
// verdict. Falls back to the first address with no verdict yet (unknown, not
// yet checked). Returns "" only when every address is known bad.
func BestEmail(r Row, verdicts map[string]Verdict) (string, *Verdict) {
	var firstUnknown string
	for _, e := range r.Emails {
		v, ok := verdicts[e]
		if !ok {
			if firstUnknown == "" {
				firstUnknown = e
			}
			continue
		}
		if v.Valid {
			vv := v
			return e, &vv
		}
	}
	return firstUnknown, nil
}
