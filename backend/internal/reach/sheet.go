package reach

import (
	"regexp"
	"strconv"
	"strings"

	"emailsender/internal/hr"
)

// The sheet is several contact lists concatenated into one tab, each with its
// own column layout, and the whole block is then repeated a second time. A
// fixed column map therefore reads the wrong cells for most of the file, so the
// parser instead detects a header row and switches layout at that point.
//
// Layouts seen in the file:
//
//	"Startup Nme | Name | Recruiter Email | personal Email | Location | Website | Industry | co-founder Email"
//	"Company Name | HR Email | Contact Number"
//	"SNo | Name | Email | Title | Company"
//	"Company Name | Role | Experience | Location | Email"
//	"Name | Email | Title | Company"
type layout struct {
	company  string // column holding the employer name
	person   string // column holding the contact's name
	title    string // column holding their job title, if any
	emails   []string // columns that may contain addresses, best first
	location string
	website  string
	industry string
	role     string // the ROLE being hired for (not the contact's own title)
	exp      string // stated experience requirement
}

// startupLayout is the file's first section, and the default before any header
// is seen.
var startupLayout = layout{
	company: "B", person: "C", emails: []string{"D", "E", "I"},
	location: "F", website: "G", industry: "H",
}

// detectLayout recognises a header row and returns the layout it introduces.
// Returns nil for ordinary data rows.
func detectLayout(row map[string]string) *layout {
	norm := func(col string) string {
		return strings.ToLower(strings.Join(strings.Fields(row[col]), " "))
	}
	a, b, c, d := norm("A"), norm("B"), norm("C"), norm("D")

	switch {
	case strings.HasPrefix(b, "startup") && strings.Contains(d, "recruiter email"):
		l := startupLayout
		return &l
	case a == "company name" && strings.Contains(b, "hr email"):
		return &layout{company: "A", emails: []string{"B"}}
	case a == "sno" && b == "name" && c == "email":
		return &layout{person: "B", emails: []string{"C"}, title: "D", company: "E"}
	case a == "company name" && b == "role" && strings.Contains(c, "experience"):
		return &layout{company: "A", role: "B", exp: "C", location: "D", emails: []string{"E"}}
	case a == "name" && b == "email" && c == "title":
		return &layout{person: "A", emails: []string{"B"}, title: "C", company: "D"}
	}
	return nil
}

// expRe pulls a stated experience requirement out of a cell, e.g.
// "0-1 year experience needed" → 0.
var expRe = regexp.MustCompile(`(?i)^\s*(\d+)\s*[-–]\s*(\d+)\s*(?:\+\s*)?(?:years?|yrs?)`)

// parseMinExp returns the lower bound of a stated experience requirement, or -1
// when the cell states none.
func parseMinExp(cell string) int {
	m := expRe.FindStringSubmatch(strings.TrimSpace(cell))
	if m == nil {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return n
}

// genericHosts are mailbox providers that carry no employer information.
var genericHosts = []string{"gmail.", "outlook.", "yahoo.", "hotmail.", "rediffmail.", "proton.", "icloud.", "live."}

// companyFromEmail recovers an employer name from an address when the sheet
// gives no usable company. "bishnu.sharma@blackbuck.com" → "Blackbuck".
// Generic mailbox providers yield "" — there is no company in a gmail address.
func companyFromEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	host := email[at+1:]
	for _, g := range genericHosts {
		if strings.HasPrefix(host, g) {
			return ""
		}
	}
	// Strip country/geo and mail sub-labels ("in.ibm.com" → "ibm",
	// "careers.dell.com" → "dell") so one company is not screened twice under
	// two spellings.
	parts := strings.Split(host, ".")
	for len(parts) > 1 {
		switch parts[0] {
		case "in", "us", "uk", "eu", "apac", "emea", "mail", "email", "careers", "jobs", "hr", "corp", "www":
			parts = parts[1:]
			continue
		}
		break
	}
	label := parts[0]
	label = strings.ReplaceAll(label, "-", " ")
	if label == "" {
		return ""
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

// looksLikeDomainFragment reports whether a cell is really a mail-domain
// fragment rather than a human-readable company name — all lowercase, no
// spaces, e.g. "in.ibm" or "oracle".
func looksLikeDomainFragment(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " &,") {
		return false
	}
	return s == strings.ToLower(s)
}

// LoadRows reads every section of the outreach sheet into rows carrying at
// least one email address, de-duplicated by address across the whole file (the
// sheet repeats its entire contents a second time, and contacts also recur
// between sections).
func LoadRows(xlsxPath string) ([]Row, error) {
	raw, err := hr.ReadSheet(xlsxPath, 0)
	if err != nil {
		return nil, err
	}

	cur := startupLayout
	seen := map[string]bool{}
	var out []Row

	for _, r := range raw {
		if l := detectLayout(r); l != nil {
			cur = *l
			continue // a header row holds no contact
		}

		// Collect addresses from this layout's email columns, best first.
		var emails []string
		local := map[string]bool{}
		for _, col := range cur.emails {
			for _, e := range splitEmails(r[col]) {
				if !local[e] {
					local[e] = true
					emails = append(emails, e)
				}
			}
		}
		if len(emails) == 0 {
			continue // nothing to send to
		}
		if seen[emails[0]] {
			continue // already have this contact
		}
		seen[emails[0]] = true

		row := Row{
			Company:     strings.TrimSpace(r[cur.company]),
			Name:        strings.TrimSpace(r[cur.person]),
			Title:       strings.TrimSpace(r[cur.title]),
			Emails:      emails,
			Location:    strings.TrimSpace(r[cur.location]),
			Website:     strings.TrimSpace(r[cur.website]),
			Industry:    strings.TrimSpace(r[cur.industry]),
			Role:        strings.TrimSpace(r[cur.role]),
			MinExpYears: -1,
		}

		// A stated experience requirement is hard data — better than any estimate.
		if cur.exp != "" {
			row.MinExpYears = parseMinExp(r[cur.exp])
		}
		// Some older rows put the requirement in the name column, with the role
		// in the company column.
		if exp := parseMinExp(row.Name); exp >= 0 {
			row.MinExpYears = exp
			if row.Role == "" {
				row.Role = row.Company
			}
			row.Name = ""
			row.Company = ""
		}
		// One section stores a bare domain fragment ("in.ibm", "ge") in both the
		// name and company columns. That is a domain, not a name or a company, so
		// route it through the same normalisation as an email domain.
		if looksLikeDomainFragment(row.Company) {
			if derived := companyFromEmail("x@" + row.Company + ".com"); derived != "" {
				row.Company = derived
			}
		}
		if looksLikeDomainFragment(row.Name) || strings.EqualFold(row.Name, row.Company) {
			row.Name = "" // not a person's name
		}

		// Fall back to the email domain when the section carries no company.
		if row.Company == "" {
			row.Company = companyFromEmail(emails[0])
		}
		if row.Company == "" {
			continue // unattributable contact — cannot screen or personalise it
		}

		out = append(out, row)
	}
	return out, nil
}

// Companies returns the distinct company names across rows, first-seen casing
// preserved — the unit the AI screen works on.
func Companies(rows []Row) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		key := NormCompany(r.Company)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r.Company)
	}
	return out
}
