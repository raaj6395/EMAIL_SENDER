// Package verify does a lightweight, safe pre-send email check: RFC-5322 syntax
// plus a DNS MX lookup to confirm the domain actually accepts mail. This catches
// the common bounce causes (typos, dead/fake domains like "@gmial.com") without
// an SMTP mailbox probe — which is unreliable (Gmail always says OK, catch-all
// domains accept everything) and slow. It never guarantees the mailbox exists;
// it only rules out addresses that clearly can't receive mail.
package verify

import (
	"context"
	"net"
	"net/mail"
	"strings"
	"time"
)

// Code classifies the result.
type Code string

const (
	CodeOK        Code = "ok"         // syntax valid + domain accepts mail
	CodeBadSyntax Code = "bad_syntax" // not a valid address — block
	CodeNoMX      Code = "no_mx"      // domain has no mail server — block
	CodeError     Code = "error"      // lookup failed (network/timeout) — allow, uncertain
)

// Result is the outcome of a check.
type Result struct {
	Email  string `json:"email"`
	Valid  bool   `json:"valid"`  // false only for clear failures (bad_syntax / no_mx)
	Code   Code   `json:"code"`
	Reason string `json:"reason"` // human-readable
}

// mxLookup is overridable in tests.
var mxLookup = func(ctx context.Context, domain string) ([]*net.MX, error) {
	var r net.Resolver
	return r.LookupMX(ctx, domain)
}

// Check validates one email address. A CodeError result is treated as "valid"
// (Valid=true) so a transient DNS hiccup never blocks a genuine address — only
// clear failures (bad syntax, no MX) return Valid=false.
func Check(ctx context.Context, email string) Result {
	addr := strings.TrimSpace(email)

	parsed, err := mail.ParseAddress(addr)
	if err != nil || !strings.Contains(parsed.Address, "@") {
		return Result{Email: addr, Valid: false, Code: CodeBadSyntax, Reason: "That doesn't look like a valid email address."}
	}
	// Use the parsed address (strips any display name).
	at := strings.LastIndex(parsed.Address, "@")
	domain := strings.ToLower(parsed.Address[at+1:])
	if domain == "" || !strings.Contains(domain, ".") {
		return Result{Email: addr, Valid: false, Code: CodeBadSyntax, Reason: "The email domain looks invalid."}
	}

	lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mx, err := mxLookup(lctx, domain)
	if err != nil {
		// Distinguish "no such host / no MX" (a real failure) from transient errors.
		var dnsErr *net.DNSError
		if isNotFound(err, &dnsErr) {
			return Result{Email: addr, Valid: false, Code: CodeNoMX,
				Reason: "The domain \"" + domain + "\" isn't set up to receive email (no mail server found). Check for a typo."}
		}
		// Transient (timeout, temporary DNS): don't block a possibly-good address.
		return Result{Email: addr, Valid: true, Code: CodeError, Reason: "Couldn't verify the domain right now — proceeding."}
	}
	if len(mx) == 0 {
		return Result{Email: addr, Valid: false, Code: CodeNoMX,
			Reason: "The domain \"" + domain + "\" has no mail server (no MX record). Check for a typo."}
	}
	return Result{Email: addr, Valid: true, Code: CodeOK, Reason: "Domain accepts mail."}
}

// isNotFound reports whether err is a definitive "domain/host not found" DNS
// error (as opposed to a temporary/timeout error).
func isNotFound(err error, target **net.DNSError) bool {
	if de, ok := err.(*net.DNSError); ok {
		*target = de
		return de.IsNotFound
	}
	return false
}
