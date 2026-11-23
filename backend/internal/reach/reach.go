// Package reach turns the startup outreach spreadsheet (data/reach.xlsx) into a
// shortlist worth emailing: rows parsed from the sheet, each email checked for
// deliverability, and each company screened by AI for whether it hires fresher
// (0-1 yr) SDEs at 12+ LPA.
//
// The sheet carries contacts only — no role and no CTC — so the screen cannot
// come from the file. It is an AI estimate per COMPANY (cached to disk, so a
// company is only ever paid for once), the same approach the targets package
// uses and for the same reason: Indian postings almost never publish salary.
package reach

import (
	"regexp"
	"strings"
)

// Row is one contact from the sheet, after parsing and de-duplication.
type Row struct {
	Company  string   `json:"company"`
	Name     string   `json:"name"`
	Emails   []string `json:"emails"` // recruiter + personal + co-founder, de-duped
	Location string   `json:"location"`
	Website  string   `json:"website"`
	Industry string   `json:"industry"`

	// The sheet holds two different layouts. The later section lists a job
	// ROLE rather than a company, with the years of experience it wants in the
	// name column — real data, so it beats any AI guess about the employer.
	Role string `json:"role,omitempty"` // e.g. "Data Analyst Intern"
	// Title is the contact's own job title, where the sheet gives one
	// (e.g. "Head HR") — useful for judging who is worth emailing.
	Title string `json:"title,omitempty"`
	// MinExpYears is the lower bound of the stated requirement; -1 when the row
	// states none. A row wanting 1+ years is not a fresher opening.
	MinExpYears int `json:"minExpYears"`
}

// IsFresherRole reports whether a role-style row is open to freshers. Rows that
// state no requirement are treated as open — the screen still has to clear them.
func (r Row) IsFresherRole() bool { return r.MinExpYears <= 0 }

// Contact is a Row joined with its verification and screening results — what
// the UI renders and sends from.
type Contact struct {
	Row
	Email    string   `json:"email"`              // the best deliverable address
	Verified *Verdict `json:"verified,omitempty"` // nil until checked
	Screen   *Screen  `json:"screen,omitempty"`   // nil until screened
}

// Verdict is the cached result of an email deliverability check.
type Verdict struct {
	Email     string `json:"email"`
	Valid     bool   `json:"valid"`
	Code      string `json:"code"`   // ok | bad_syntax | no_mx | no_mailbox
	Reason    string `json:"reason"`
	CheckedAt string `json:"checkedAt"` // RFC3339

	// SMTP mailbox probe, filled in for shortlisted contacts only. Probing is
	// slow and must be rate-limited, so it runs on the final list rather than
	// across the whole sheet.
	Probe       string `json:"probe,omitempty"` // deliverable | rejected | catch_all | unknown
	ProbeReason string `json:"probeReason,omitempty"`
	ProbedAt    string `json:"probedAt,omitempty"`
}

// Confirmed reports whether an SMTP probe positively accepted this mailbox.
func (v Verdict) Confirmed() bool { return v.Probe == string(ProbeDeliverable) }

// Screen is the cached AI judgement for one company.
type Screen struct {
	Company    string  `json:"company"`
	HiresSDE   bool    `json:"hiresSDE"`   // hires software engineers at all
	Fresher    bool    `json:"fresher"`    // hires 0-1 yr engineers
	MinLPA     float64 `json:"minLPA"`     // typical fresher CTC band, in lakhs
	MaxLPA     float64 `json:"maxLPA"`
	Confidence string  `json:"confidence"` // high | medium | low
	Note       string  `json:"note"`       // one line: what they do
	ScreenedAt string  `json:"screenedAt"` // RFC3339
}

// MidLPA is the midpoint of the estimated band, used for the floor test and
// for sorting best-paying first.
func (s Screen) MidLPA() float64 {
	if s.MaxLPA <= 0 {
		return s.MinLPA
	}
	return (s.MinLPA + s.MaxLPA) / 2
}

// FloorLPA is the CTC floor for the shortlist. A company qualifies only when
// the MIDPOINT of its estimated fresher band reaches this — so "12+" means a
// typical offer around 12, not a band that merely tops out there.
const FloorLPA = 12.0

// Qualifies reports whether a screened company belongs on the shortlist: it
// hires freshers into SDE roles and pays at least the floor.
func (s Screen) Qualifies() bool {
	return s.HiresSDE && s.Fresher && s.MidLPA() >= FloorLPA
}

// emailRe pulls addresses out of a cell that may hold several, separated by
// newlines, commas or spaces (the sheet's recruiter column commonly has two).
var emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// splitEmails extracts every address in a cell, lowercased.
func splitEmails(cell string) []string {
	found := emailRe.FindAllString(cell, -1)
	out := make([]string, 0, len(found))
	for _, e := range found {
		out = append(out, strings.ToLower(strings.TrimSpace(e)))
	}
	return out
}

// NormCompany is the cache key for a company name.
func NormCompany(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
