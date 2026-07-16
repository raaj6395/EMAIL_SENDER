package batch

import (
	"strings"
)

// ParseRows turns pasted text into batch items. Each non-empty line is one
// recipient in one of these forms (comma- or tab-separated):
//
//	priya@stripe.com
//	priya@stripe.com, Stripe
//	priya@stripe.com, Stripe, Priya
//
// A missing company is guessed from the email domain. Lines without a valid
// email are skipped. Duplicate emails (case-insensitive) are dropped.
func ParseRows(text string) []Item {
	var items []Item
	seen := map[string]bool{}

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// Split on comma or tab.
		fields := splitFields(line)
		email := strings.TrimSpace(fields[0])
		if !validEmail(email) {
			continue
		}
		key := strings.ToLower(email)
		if seen[key] {
			continue
		}
		seen[key] = true

		company := ""
		name := ""
		if len(fields) > 1 {
			company = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			name = strings.TrimSpace(fields[2])
		}
		if company == "" {
			company = GuessCompany(email)
		}
		items = append(items, Item{Email: email, Company: company, Name: name})
	}
	return items
}

func splitFields(line string) []string {
	// Prefer comma; fall back to tab if there are no commas.
	if strings.Contains(line, ",") {
		return strings.Split(line, ",")
	}
	if strings.Contains(line, "\t") {
		return strings.Split(line, "\t")
	}
	return []string{line}
}

func validEmail(s string) bool {
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	dom := s[at+1:]
	return strings.Contains(dom, ".") && !strings.ContainsAny(s, " \t")
}

// GuessCompany derives a company name from an email domain, mirroring the
// frontend's guess: drop common providers, take the second-level domain, and
// title-case it (stripe.com → "Stripe", careers.acme.co.uk → "Acme").
func GuessCompany(email string) string {
	at := strings.Index(email, "@")
	if at < 0 {
		return ""
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if domain == "" {
		return ""
	}
	// Skip generic mailbox providers — no company signal there.
	if genericProviders[domain] {
		return ""
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return ""
	}
	// Choose the registrable label: for multi-part TLDs (co.uk, com.au) take the
	// third-from-last, else second-from-last.
	idx := len(labels) - 2
	if len(labels) >= 3 && secondLevelTLDs[labels[len(labels)-2]+"."+labels[len(labels)-1]] {
		idx = len(labels) - 3
	}
	if idx < 0 {
		idx = 0
	}
	label := labels[idx]
	// Strip a leading "careers"/"jobs"/"mail" style subdomain if it slipped in.
	return titleCase(label)
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	// Split on hyphens so "info-edge" → "Info Edge".
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

var genericProviders = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "yahoo.com": true, "yahoo.co.in": true,
	"outlook.com": true, "hotmail.com": true, "live.com": true, "icloud.com": true,
	"proton.me": true, "protonmail.com": true, "aol.com": true, "zoho.com": true,
	"rediffmail.com": true, "mail.com": true,
}

var secondLevelTLDs = map[string]bool{
	"co.uk": true, "com.au": true, "co.in": true, "co.jp": true, "com.br": true,
	"co.nz": true, "com.sg": true, "co.za": true,
}
