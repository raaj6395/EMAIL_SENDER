// Package targets builds a list of companies worth cold-emailing as a fresher
// SDE in India — hypergrowth startups, product MNCs, and global capability
// centres that pay above a chosen CTC floor.
//
// Why this isn't a job feed: Indian job postings almost never publish salary
// (measured at ~8% coverage on LinkedIn-sourced fresher roles, and under 5% in
// INR), so a ">12 LPA" filter cannot be derived from postings. What IS stable
// and well-known is what a given COMPANY pays a fresher. So this package works
// at the company level: it asks the model to name companies matching a target
// profile, scores each for typical fresher CTC and company type, and caches the
// result to disk so repeat loads are instant and free.
//
// Everything here is an ESTIMATE from the model's knowledge, not live market
// data — the UI labels it as such. It is meant to answer "who should I target",
// not "what is this exact offer worth".
package targets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Category is the kind of employer, which shapes both comp and how you pitch.
const (
	CatHypergrowth = "hypergrowth" // fast-scaling startups / unicorns
	CatMNC         = "mnc"         // global product companies with India offices
	CatGCC         = "gcc"         // global capability / R&D centres of non-tech firms
	CatProduct     = "product"     // established Indian product companies
	CatFintech     = "fintech"
	CatOther       = "other"
)

// AllCategories is the display order used by the UI filter.
var AllCategories = []string{CatHypergrowth, CatMNC, CatGCC, CatProduct, CatFintech}

// FloorLPA is the hard minimum typical fresher CTC for a company to be worth
// cold-emailing at all. The model is asked for companies above it, but asking
// is not enforcing — it occasionally returns a plausible-sounding company that
// pays well under the floor. So every path into the cache (discover, manual
// add) and every read out of it applies this, and the UI cannot lower it.
const FloorLPA = 12.0

// ClearsFloor reports whether a company's typical offer reaches FloorLPA.
func (c Company) ClearsFloor() bool { return c.MidLPA() >= FloorLPA }

// keepAboveFloor drops companies whose typical offer is under the floor.
func keepAboveFloor(cs []Company) []Company {
	out := cs[:0:0]
	for _, c := range cs {
		if c.ClearsFloor() {
			out = append(out, c)
		}
	}
	return out
}

// CategoryLabel renders a category for the UI.
func CategoryLabel(c string) string {
	switch c {
	case CatHypergrowth:
		return "Hypergrowth startup"
	case CatMNC:
		return "MNC / Big Tech"
	case CatGCC:
		return "GCC / R&D centre"
	case CatProduct:
		return "Indian product co."
	case CatFintech:
		return "Fintech"
	default:
		return "Other"
	}
}

// Company is one target employer with the model's estimate of what it pays a
// fresher SDE in India.
type Company struct {
	Name string `json:"name"`
	// MinLPA/MaxLPA bound the typical fresher total CTC in lakhs per annum.
	MinLPA   float64 `json:"minLPA"`
	MaxLPA   float64 `json:"maxLPA"`
	Category string  `json:"category"`
	// Confidence is how sure the model is about the comp band: "high" for
	// companies with well-known public comp data, "low" for obscure ones.
	Confidence  string   `json:"confidence"`
	HQ          string   `json:"hq"`          // primary India hiring city
	Note        string   `json:"note"`        // one line: what they do / why target them
	CareersURL  string   `json:"careersUrl"`  // best-effort careers page
	EmailDomain string   `json:"emailDomain"` // for guessing HR addresses, e.g. "stripe.com"
	Tech        []string `json:"tech"`        // notable stack, helps tailor the pitch
	AddedAt     string   `json:"addedAt"`     // RFC3339, when it entered the cache
	Source      string   `json:"source"`      // "ai" | "manual"
}

// MidLPA is the midpoint of the band, used for sorting.
func (c Company) MidLPA() float64 {
	if c.MaxLPA <= 0 {
		return c.MinLPA
	}
	return (c.MinLPA + c.MaxLPA) / 2
}

// store is the on-disk shape: a map of normalized name → company.
type store map[string]Company

var mu sync.Mutex

func normName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func loadLocked(path string) (store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store{}, nil
		}
		return nil, err
	}
	var s store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s == nil {
		s = store{}
	}
	return s, nil
}

func saveLocked(path string, s store) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Filter narrows a company list for the UI.
type Filter struct {
	MinLPA float64 // only companies whose band reaches at least this; values
	// below FloorLPA are raised to it — the floor is not user-lowerable
	Categories []string // empty = all
	Query      string   // substring match on name/note/tech
}

// Load returns all cached companies matching the filter, best-paying first.
func Load(path string, f Filter) ([]Company, error) {
	mu.Lock()
	defer mu.Unlock()

	s, err := loadLocked(path)
	if err != nil {
		return nil, err
	}

	// The floor is structural: a caller passing 0 (or omitting minLPA) still
	// gets only companies above it.
	minLPA := f.MinLPA
	if minLPA < FloorLPA {
		minLPA = FloorLPA
	}

	catSet := map[string]bool{}
	for _, c := range f.Categories {
		catSet[strings.ToLower(strings.TrimSpace(c))] = true
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))

	out := make([]Company, 0, len(s))
	for _, c := range s {
		// A company qualifies when the MIDPOINT of its band clears the floor, so
		// "pays at least 25" means a typical offer around 25+, not a company whose
		// band merely tops out there (22-30 is really a ~26 company; 18-30 is not).
		if c.MidLPA() < minLPA {
			continue
		}
		if len(catSet) > 0 && !catSet[c.Category] {
			continue
		}
		if q != "" && !matchesQuery(c, q) {
			continue
		}
		out = append(out, c)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if a, b := out[i].MidLPA(), out[j].MidLPA(); a != b {
			return a > b
		}
		return normName(out[i].Name) < normName(out[j].Name)
	})
	return out, nil
}

func matchesQuery(c Company, q string) bool {
	if strings.Contains(strings.ToLower(c.Name), q) ||
		strings.Contains(strings.ToLower(c.Note), q) ||
		strings.Contains(strings.ToLower(c.HQ), q) {
		return true
	}
	for _, t := range c.Tech {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// Count returns how many companies are cached.
func Count(path string) int {
	mu.Lock()
	defer mu.Unlock()
	s, err := loadLocked(path)
	if err != nil {
		return 0
	}
	return len(s)
}

// Remove deletes a company from the cache.
func Remove(path, name string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := loadLocked(path)
	if err != nil {
		return err
	}
	delete(s, normName(name))
	return saveLocked(path, s)
}

// upsertLocked merges companies into the store, keeping existing entries unless
// overwrite is set. Returns how many were newly added.
func upsertLocked(s store, cs []Company, overwrite bool) int {
	added := 0
	now := time.Now().Format(time.RFC3339)
	for _, c := range cs {
		c.Name = strings.TrimSpace(c.Name)
		if c.Name == "" {
			continue
		}
		key := normName(c.Name)
		if existing, ok := s[key]; ok && !overwrite {
			continue
		} else if ok {
			c.AddedAt = existing.AddedAt // preserve original discovery time
		}
		if c.AddedAt == "" {
			c.AddedAt = now
		}
		if c.Category == "" {
			c.Category = CatOther
		}
		if c.Source == "" {
			c.Source = "ai"
		}
		if _, existed := s[key]; !existed {
			added++
		}
		s[key] = c
	}
	return added
}

// Discover asks the model for companies matching the target profile and merges
// the new ones into the cache. Companies already cached are not re-queried, and
// their names are passed to the model as an exclusion list so each run surfaces
// fresh ones. Returns the newly added companies.
func Discover(ctx context.Context, path, apiKey, model string, opts DiscoverOptions) ([]Company, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI key not configured: set OPENAI_API_KEY in backend/.env to build the target list")
	}

	mu.Lock()
	s, err := loadLocked(path)
	if err != nil {
		mu.Unlock()
		return nil, err
	}
	known := make([]string, 0, len(s))
	for _, c := range s {
		known = append(known, c.Name)
	}
	mu.Unlock() // don't hold the lock across a slow network call

	found, err := discoverBatch(ctx, apiKey, model, opts, known)
	if err != nil {
		return nil, err
	}
	// The prompt asks for companies above the floor, but the model sometimes
	// returns one anyway. Drop them here so they never enter the cache.
	found = keepAboveFloor(found)

	mu.Lock()
	defer mu.Unlock()
	// Re-read: another request may have written while we were on the network.
	s, err = loadLocked(path)
	if err != nil {
		return nil, err
	}
	var fresh []Company
	for _, c := range found {
		if _, exists := s[normName(c.Name)]; !exists {
			fresh = append(fresh, c)
		}
	}
	upsertLocked(s, found, false)
	if err := saveLocked(path, s); err != nil {
		return nil, err
	}
	return fresh, nil
}

// AddManual scores a user-supplied company name and stores it. Existing entries
// are refreshed rather than duplicated.
func AddManual(ctx context.Context, path, apiKey, model, name string) (Company, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Company{}, fmt.Errorf("company name is required")
	}
	if apiKey == "" {
		return Company{}, fmt.Errorf("OpenAI key not configured: set OPENAI_API_KEY in backend/.env")
	}

	found, err := scoreNamed(ctx, apiKey, model, []string{name})
	if err != nil {
		return Company{}, err
	}
	if len(found) == 0 {
		return Company{}, fmt.Errorf("could not find comp information for %q", name)
	}
	c := found[0]
	c.Source = "manual"
	if !c.ClearsFloor() {
		return Company{}, fmt.Errorf("%s pays freshers about %.0f-%.0f LPA, below the %.0f LPA floor — not added",
			c.Name, c.MinLPA, c.MaxLPA, FloorLPA)
	}

	mu.Lock()
	defer mu.Unlock()
	s, err := loadLocked(path)
	if err != nil {
		return Company{}, err
	}
	upsertLocked(s, []Company{c}, true)
	if err := saveLocked(path, s); err != nil {
		return Company{}, err
	}
	return s[normName(c.Name)], nil
}
