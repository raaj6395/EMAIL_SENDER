package verify

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
)

// The bounce list holds addresses that already hard-bounced ("Address not
// found"). It is the only reliable existence signal for Google/Microsoft
// mailboxes, which accept every address at SMTP time, so Check consults it
// before anything else and a dead address bounces at most once.
var (
	bouncedMu   sync.RWMutex
	bounced     = map[string]bool{}
	bouncedPath string
)

// LoadBounced reads the bounce list from disk; a missing file is an empty list.
func LoadBounced(path string) error {
	bouncedMu.Lock()
	defer bouncedMu.Unlock()
	bouncedPath = path
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, e := range list {
		bounced[strings.ToLower(e)] = true
	}
	return nil
}

// AddBounced records addresses and persists the list. Returns how many were new.
func AddBounced(emails []string) (int, error) {
	bouncedMu.Lock()
	defer bouncedMu.Unlock()
	added := 0
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" && !bounced[e] {
			bounced[e] = true
			added++
		}
	}
	if added == 0 || bouncedPath == "" {
		return added, nil
	}
	list := make([]string, 0, len(bounced))
	for e := range bounced {
		list = append(list, e)
	}
	sort.Strings(list)
	b, _ := json.MarshalIndent(list, "", "  ")
	tmp := bouncedPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return added, err
	}
	return added, os.Rename(tmp, bouncedPath)
}

func isBounced(email string) bool {
	bouncedMu.RLock()
	defer bouncedMu.RUnlock()
	return bounced[strings.ToLower(email)]
}
