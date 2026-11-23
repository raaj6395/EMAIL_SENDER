package exists

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		in   reacher
		want string
	}{
		{"safe", reacher{IsReachable: "safe"}, "exist"},
		{"invalid", reacher{IsReachable: "invalid"}, "invalid"},
		{"disabled", reacher{IsReachable: "safe", SMTP: smtpResult{IsDisabled: true}}, "invalid"},
		{"risky role", reacher{IsReachable: "risky", SMTP: smtpResult{IsDeliverable: true}}, "exist"},
		{"full inbox", reacher{IsReachable: "risky", SMTP: smtpResult{HasFullInbox: true}}, "exist"},
		{"catch-all", reacher{IsReachable: "risky", SMTP: smtpResult{IsDeliverable: true, IsCatchAll: true}}, "unknown"},
		{"unknown", reacher{IsReachable: "unknown"}, "unknown"},
	}
	for _, tc := range cases {
		if got := classify(tc.in); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestSlowCheckDoesNotAbortBatch(t *testing.T) {
	old := checkTimeout
	checkTimeout = 50 * time.Millisecond
	t.Cleanup(func() { checkTimeout = old })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			To string `json:"to_email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.To == "slow@x.com" {
			time.Sleep(300 * time.Millisecond)
		}
		_ = json.NewEncoder(w).Encode(reacher{IsReachable: "safe"})
	}))
	defer srv.Close()

	var mu sync.Mutex
	got := map[string]string{}
	err := CheckAll(context.Background(), srv.URL, []Addr{
		{Email: "slow@x.com"},
		{Email: "ok@x.com", Company: "Ok"},
	}, func(a Addr, kind string) {
		mu.Lock()
		got[a.Email] = kind
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["ok@x.com"] != "exist" || got["slow@x.com"] != "unknown" {
		t.Fatalf("got %+v", got)
	}
}

func TestFastResultStreamsBeforeSlowFinishes(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			To string `json:"to_email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.To == "slow@x.com" {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_ = json.NewEncoder(w).Encode(reacher{IsReachable: "safe"})
	}))
	defer srv.Close()

	fast := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = CheckAll(context.Background(), srv.URL, []Addr{
			{Email: "slow@x.com"},
			{Email: "ok@x.com"},
		}, func(a Addr, kind string) {
			if a.Email == "ok@x.com" && kind == "exist" {
				close(fast)
			}
		})
	}()

	select {
	case <-fast:
	case <-time.After(2 * time.Second):
		t.Fatal("fast address was not reported while the slow one was still running")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("check did not finish")
	}
}
