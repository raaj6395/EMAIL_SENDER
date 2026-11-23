package reach

import (
	"context"

	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// SMTP mailbox probing: connect to the domain's mail server and ask, via RCPT
// TO, whether an address is deliverable — without ever sending a message.
//
// This is a genuine existence check, but it is not universally conclusive:
//
//   - Google and Microsoft accept every RCPT and bounce later, so addresses on
//     their infrastructure can only come back "unknown".
//   - Catch-all domains accept every address by design; detected by probing a
//     random address first and seeing whether it is also accepted.
//   - Some servers greylist or rate-limit, which is a soft failure, not proof
//     of absence.
//
// Anything not positively rejected therefore stays "unknown" rather than being
// called valid, so a probe never invents confidence it does not have.

// ProbeCode classifies a mailbox probe.
type ProbeCode string

const (
	ProbeDeliverable ProbeCode = "deliverable" // server accepted the recipient
	ProbeRejected    ProbeCode = "rejected"    // server refused: mailbox does not exist
	ProbeCatchAll    ProbeCode = "catch_all"   // domain accepts everything — unprovable
	ProbeUnknown     ProbeCode = "unknown"     // greylisted, blocked, or accept-all provider
)

// probeFrom is the MAIL FROM used for probes. It must be a real, deliverable
// address on a domain you control: servers reject probes from bogus senders,
// and a null sender is treated as a bounce by some. It is never sent mail.
var probeFrom = "verify@gmail.com"

// SetProbeSender overrides the MAIL FROM address used for probes.
func SetProbeSender(addr string) {
	if strings.Contains(addr, "@") {
		probeFrom = addr
	}
}

// randomLocal is an address that cannot plausibly exist, used to detect a
// domain that accepts everything.
const randomLocal = "zz-no-such-user-9f3a71c4"

// domainProbe holds what we learned about one domain, so every address on it
// reuses a single connection's findings.
type domainProbe struct {
	catchAll bool
	err      error
}

// ProbeMailbox checks one address against its domain's mail server.
// `catchAllHint` avoids re-testing a domain already known to accept everything.
func ProbeMailbox(ctx context.Context, email string, catchAllHint *domainProbe) (ProbeCode, string) {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ProbeUnknown, "not an address"
	}
	domain := strings.ToLower(email[at+1:])

	mxs, err := net.DefaultResolver.LookupMX(ctx, domain)
	if err != nil || len(mxs) == 0 {
		return ProbeUnknown, "no MX to probe"
	}
	host := strings.TrimSuffix(mxs[0].Host, ".")

	// Providers that accept every recipient and bounce afterwards cannot be
	// probed; say so rather than reporting a meaningless "accepted".
	if isAcceptAllProvider(host) {
		return ProbeUnknown, "provider accepts all recipients at SMTP time (Google/Microsoft)"
	}

	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", host+":25")
	if err != nil {
		return ProbeUnknown, "could not reach mail server: " + err.Error()
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(25 * time.Second))

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return ProbeUnknown, "SMTP handshake failed"
	}
	defer c.Close()

	if err := c.Hello("gmail.com"); err != nil {
		return ProbeUnknown, "EHLO refused"
	}
	if err := c.Mail(probeFrom); err != nil {
		return ProbeUnknown, "MAIL FROM refused"
	}

	// Does this domain accept anything at all? Test once per domain.
	if catchAllHint != nil && !catchAllHint.catchAll {
		if err := c.Rcpt(randomLocal + "@" + domain); err == nil {
			catchAllHint.catchAll = true
			return ProbeCatchAll, "domain accepts every address — cannot confirm this mailbox"
		}
	}

	err = c.Rcpt(email)
	if err == nil {
		return ProbeDeliverable, "mail server accepted this recipient"
	}
	msg := err.Error()
	// A 5xx that refuses the CLIENT (blocklisted IP, reputation, policy) says
	// nothing about the mailbox. Treating it as "does not exist" would delete
	// perfectly good contacts, so it must be classified as unknown.
	if isSenderBlocked(msg) {
		return ProbeUnknown, "our IP was blocked by this server, mailbox not tested: " + truncateMsg(msg)
	}
	// 5xx = permanent refusal of the RECIPIENT, i.e. the mailbox does not exist.
	// 4xx = temporary (greylisting, rate limit) and proves nothing.
	if len(msg) > 0 && msg[0] == '5' {
		return ProbeRejected, "mail server rejected this recipient: " + truncateMsg(msg)
	}
	return ProbeUnknown, "inconclusive: " + truncateMsg(msg)
}

// senderBlockMarkers appear in 5xx replies that reject the connecting client
// rather than the recipient — blocklists, reputation and policy refusals.
var senderBlockMarkers = []string{
	"spamhaus", "spamcop", "barracudacentral", "sorbs", "blocklist", "blacklist",
	"blocked using", "client host", "access denied", "not allowed to send",
	"reputation", "poor reputation", "5.7.1", "policy reasons", "rejected due to",
	"dnsbl", "rbl", "banned sending ip", "ip is listed",
}

// isSenderBlocked reports whether a refusal targets our connecting IP/sender
// rather than the recipient address.
func isSenderBlocked(msg string) bool {
	m := strings.ToLower(msg)
	// "Unknown user" style wording is about the recipient even if a 5.7.1 code
	// happens to appear, so recipient evidence wins.
	for _, r := range []string{"unknown user", "no such user", "user unknown",
		"recipient address rejected", "does not exist", "invalid recipient",
		"mailbox unavailable", "no such recipient", "unrouteable address"} {
		if strings.Contains(m, r) {
			return false
		}
	}
	for _, b := range senderBlockMarkers {
		if strings.Contains(m, b) {
			return true
		}
	}
	return false
}

// isAcceptAllProvider reports whether a mail host is known to accept every
// recipient at SMTP time, making a probe uninformative.
func isAcceptAllProvider(host string) bool {
	h := strings.ToLower(host)
	for _, p := range []string{
		"google.com", "googlemail.com", // Gmail / Workspace
		"outlook.com", "protection.outlook.com", "hotmail.com", // Microsoft 365
		"pphosted.com", "ppe-hosted.com", // Proofpoint
		"mimecast.com", "barracudanetworks.com",
	} {
		if strings.Contains(h, p) {
			return true
		}
	}
	return false
}

func truncateMsg(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

// result is one probed address.
type result struct {
	email  string
	code   ProbeCode
	reason string
}

// ProbeBatch probes many addresses, grouped by domain so each domain is
// contacted by one goroutine at a time — that is what keeps this from looking
// like an attack and getting the IP blocklisted. Results merge into the cache
// as an extra field on the existing verdict.
func ProbeBatch(ctx context.Context, path string, emails []string, workers int, pause time.Duration) (map[string]ProbeCode, error) {
	byDomain := map[string][]string{}
	for _, e := range emails {
		at := strings.LastIndex(e, "@")
		if at < 0 {
			continue
		}
		d := strings.ToLower(e[at+1:])
		byDomain[d] = append(byDomain[d], e)
	}

	if workers <= 0 {
		workers = 8
	}
	results := make(chan result, len(emails))

	domains := make([]string, 0, len(byDomain))
	for d := range byDomain {
		domains = append(domains, d)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	// Probing thousands of addresses takes a long time, so results are flushed
	// to disk periodically rather than only at the end — an interrupted run
	// keeps everything it had already learned.
	flushDone := make(chan struct{})
	pending := make(chan result, len(emails))
	go func() {
		defer close(flushDone)
		buf := map[string]result{}
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case r, ok := <-pending:
				if !ok {
					persistProbes(path, buf)
					return
				}
				buf[r.email] = r
			case <-tick.C:
				if len(buf) > 0 {
					persistProbes(path, buf)
					buf = map[string]result{}
				}
			}
		}
	}()
	for _, d := range domains {
		wg.Add(1)
		go func(domain string, addrs []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			hint := &domainProbe{}
			for _, e := range addrs {
				if ctx.Err() != nil {
					return
				}
				code, reason := ProbeMailbox(ctx, e, hint)
				results <- result{e, code, reason}
				pending <- result{e, code, reason}
				// Sequential within a domain, with a pause — polite, and the
				// single most effective guard against being rate-limited.
				if pause > 0 {
					time.Sleep(pause)
				}
			}
		}(d, byDomain[d])
	}
	wg.Wait()
	close(results)
	close(pending)
	<-flushDone

	out := map[string]ProbeCode{}
	reasons := map[string]string{}
	for r := range results {
		out[r.email] = r.code
		reasons[r.email] = r.reason
	}

	_ = reasons
	return out, nil
}

// persistProbes merges a batch of probe results into the cache on disk.
func persistProbes(path string, buf map[string]result) {
	if len(buf) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	c, err := loadLocked(path)
	if err != nil {
		return
	}
	now := time.Now().Format(time.RFC3339)
	for email, r := range buf {
		v := c.Verdicts[email]
		v.Email = email
		v.Probe = string(r.code)
		v.ProbeReason = r.reason
		v.ProbedAt = now
		// A hard rejection of the RECIPIENT is proof the mailbox does not exist —
		// the only probe outcome strong enough to overrule the MX-based verdict.
		if r.code == ProbeRejected {
			v.Valid = false
			v.Code = "no_mailbox"
			v.Reason = r.reason
		}
		c.Verdicts[email] = v
	}
	_ = saveLocked(path, c)
}
