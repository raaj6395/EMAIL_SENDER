// Package exists asks a local check-if-email-exists backend
// (https://github.com/reacherhq/check-if-email-exists) which addresses a mail
// server will actually accept.
//
// Start it once, on a port that is not this app's:
//
//	docker run -d --name reacher -p 8081:8080 reacherhq/backend:latest
//
// Only addresses the server confirms are returned. Catch-all domains and
// "unknown" answers (Gmail often refuses to say) are left out — they are not
// proof the mailbox exists. Outbound port 25 has to be open or every check
// comes back unknown.
package exists

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const workers = 4

// checkTimeout is how long one address may take. A mail server that never
// answers must not fail the rest of the list.
var checkTimeout = 3 * time.Minute

// Addr is one pasted recipient. Company and name are carried through so the
// ones that exist can be queued without re-parsing.
type Addr struct {
	Email   string
	Company string
	Name    string
}

type smtpResult struct {
	IsDeliverable bool `json:"is_deliverable"`
	IsDisabled    bool `json:"is_disabled"`
	IsCatchAll    bool `json:"is_catch_all"`
	HasFullInbox  bool `json:"has_full_inbox"`
}

type reacher struct {
	IsReachable string     `json:"is_reachable"`
	SMTP        smtpResult `json:"smtp"`
}

// classify maps one Reacher response to exist, invalid, or unknown.
func classify(r reacher) string {
	if r.IsReachable == "invalid" || r.SMTP.IsDisabled {
		return "invalid"
	}
	if r.IsReachable == "safe" {
		return "exist"
	}
	// Risky covers role accounts and full inboxes (the mailbox is real) and
	// catch-alls (which prove nothing about this particular address).
	if r.IsReachable == "risky" && !r.SMTP.IsCatchAll && (r.SMTP.IsDeliverable || r.SMTP.HasFullInbox) {
		return "exist"
	}
	return "unknown"
}

// CheckAll checks each address against the Reacher backend at baseURL
// (default http://127.0.0.1:8081). onResult is called as soon as each address
// finishes, from the worker that checked it. A dead backend is one error;
// one address that times out is reported as "unknown" and the rest continue.
func CheckAll(ctx context.Context, baseURL string, addrs []Addr, onResult func(Addr, string)) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081"
	}
	client := &http.Client{Timeout: checkTimeout}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	var mu sync.Mutex
	var fatal error

	report := func(a Addr, kind string) {
		if onResult != nil {
			onResult(a, kind)
		}
	}

	for _, a := range addrs {
		wg.Add(1)
		go func(a Addr) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			kind, err := checkOne(ctx, client, baseURL, a.Email)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if !isDown(err) {
					report(a, "unknown")
					return
				}
				mu.Lock()
				if fatal == nil {
					fatal = err
				}
				mu.Unlock()
				cancel()
				return
			}
			report(a, kind)
		}(a)
	}
	wg.Wait()
	if fatal != nil {
		return fatal
	}
	return ctx.Err()
}

func checkOne(ctx context.Context, client *http.Client, baseURL, email string) (string, error) {
	body, _ := json.Marshal(map[string]string{"to_email": email})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v0/check_email", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if isDown(err) {
			return "", fmt.Errorf("check-if-email-exists is not running at %s. Start it with: docker run -d --name reacher -p 8081:8080 reacherhq/backend:latest", baseURL)
		}
		return "", fmt.Errorf("could not reach check-if-email-exists: %s", err.Error())
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return "", fmt.Errorf("check-if-email-exists returned %s: %s", resp.Status, msg)
	}
	var parsed reacher
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("check-if-email-exists returned unexpected JSON")
	}
	return classify(parsed), nil
}

func isDown(err error) bool {
	s := err.Error()
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "is not running")
}
