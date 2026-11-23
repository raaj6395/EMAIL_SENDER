package reach

import (
	"sort"
	"strings"
)

// Filter narrows the shortlist for the UI.
type Filter struct {
	MinLPA float64 // band midpoint floor; values under FloorLPA are raised to it
	// OnlyReady keeps just the rows ready to email: a deliverable address AND a
	// company that passed the screen. Off shows everything, so progress is
	// visible while the pipeline is still filling in.
	OnlyReady bool
	Query     string // substring match on company / name / email / industry
	Exclude   map[string]bool // contact keys already emailed
}

// Build joins rows with their cached verdicts and screens, applies the filter,
// and sorts best-paying first. It also returns the progress counters.
func Build(rows []Row, verdicts map[string]Verdict, screens map[string]Screen, f Filter) ([]Contact, Progress) {
	// The floor is structural: a caller passing 0 still gets only 12+ companies.
	minLPA := f.MinLPA
	if minLPA < FloorLPA {
		minLPA = FloorLPA
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))

	prog := Progress{Rows: len(rows), Companies: len(Companies(rows))}
	// Count verification progress per row: a row is "verified" once it has a
	// definite answer — a good address, or every address checked and bad.
	for _, r := range rows {
		if rowVerified(r, verdicts) {
			prog.Verified++
		} else {
			prog.PendingVerify++
		}
	}
	for _, name := range Companies(rows) {
		if _, ok := screens[NormCompany(name)]; ok {
			prog.Screened++
		} else {
			prog.PendingScreen++
		}
	}

	out := make([]Contact, 0, len(rows))
	for _, r := range rows {
		// A row stating it wants 1+ years is not a fresher opening. This comes
		// from the sheet itself, so it outranks any AI estimate.
		if !r.IsFresherRole() {
			continue
		}
		email, verdict := BestEmail(r, verdicts)
		if email == "" {
			continue // every address is known undeliverable — never show it
		}
		screen, screened := screens[NormCompany(r.Company)]

		// A screened company that fails the bar is out, whatever the filter says —
		// this list exists to be the 12+ LPA fresher-SDE shortlist.
		if screened && (!screen.Qualifies() || screen.MidLPA() < minLPA) {
			continue
		}
		ready := verdict != nil && verdict.Valid && screened
		if f.OnlyReady && !ready {
			continue
		}
		if f.Exclude[email] {
			continue // already emailed
		}

		c := Contact{Row: r, Email: email, Verified: verdict}
		if screened {
			s := screen
			c.Screen = &s
		}
		if q != "" && !matches(c, q) {
			continue
		}
		out = append(out, c)
	}

	// Best-paying first; screened rows ahead of unscreened ones so the vetted
	// shortlist is what you see at the top.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := midOf(out[i]), midOf(out[j])
		if a != b {
			return a > b
		}
		return strings.ToLower(out[i].Company) < strings.ToLower(out[j].Company)
	})

	prog.Shortlisted = len(out)
	return out, prog
}

// rowVerified reports whether a row has a definite verification answer.
func rowVerified(r Row, verdicts map[string]Verdict) bool {
	for _, e := range r.Emails {
		v, ok := verdicts[e]
		if !ok {
			return false // still an unchecked address to try
		}
		if v.Valid {
			return true
		}
	}
	return true // all addresses checked, all bad
}

// midOf is a contact's sort key: its band midpoint, or -1 when unscreened so
// screened rows sort above the not-yet-judged ones.
func midOf(c Contact) float64 {
	if c.Screen == nil {
		return -1
	}
	return c.Screen.MidLPA()
}

func matches(c Contact, q string) bool {
	hay := strings.ToLower(strings.Join([]string{c.Company, c.Name, c.Email, c.Industry, c.Location}, " "))
	return strings.Contains(hay, q)
}
