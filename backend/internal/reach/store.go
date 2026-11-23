package reach

import (
	"encoding/json"
	"os"
	"sync"
)

// cache is the on-disk state for the pipeline: what we've verified and what
// we've screened. Both are keyed maps so a rerun only pays for what's new —
// the sheet has ~7.5k rows, so re-verifying or re-screening everything on each
// page load would be both slow and (for the AI half) expensive.
type cache struct {
	// Verdicts maps a lowercased email → its deliverability result.
	Verdicts map[string]Verdict `json:"verdicts"`
	// Screens maps a normalized company name → its AI judgement.
	Screens map[string]Screen `json:"screens"`
}

var mu sync.Mutex

func loadLocked(path string) (*cache, error) {
	c := &cache{Verdicts: map[string]Verdict{}, Screens: map[string]Screen{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	if c.Verdicts == nil {
		c.Verdicts = map[string]Verdict{}
	}
	if c.Screens == nil {
		c.Screens = map[string]Screen{}
	}
	return c, nil
}

func saveLocked(path string, c *cache) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// Write-then-rename: a crash mid-write can't leave a truncated cache behind,
	// which would silently discard hours of verification and paid AI screening.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load returns the cached verdicts and screens.
func Load(path string) (map[string]Verdict, map[string]Screen, error) {
	mu.Lock()
	defer mu.Unlock()
	c, err := loadLocked(path)
	if err != nil {
		return nil, nil, err
	}
	return c.Verdicts, c.Screens, nil
}

// Progress reports how much of the sheet has been processed, for the UI.
type Progress struct {
	Rows            int `json:"rows"`            // contact rows in the sheet
	Companies       int `json:"companies"`       // distinct companies
	Verified        int `json:"verified"`        // emails checked so far
	PendingVerify   int `json:"pendingVerify"`   // emails still unchecked
	Screened        int `json:"screened"`        // companies screened so far
	PendingScreen   int `json:"pendingScreen"`   // companies still unscreened
	Shortlisted     int `json:"shortlisted"`     // rows passing every filter
}
