package hr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"

// rankBatchSize is how many companies we score per OpenAI call. Larger batches
// mean fewer calls; 60 keeps each prompt/response comfortably small.
const rankBatchSize = 60

// maxNewPerCall bounds how many not-yet-cached companies a single (non-forced)
// request will score, so the first load of a large sheet (~1500 companies)
// never blocks the HTTP request for minutes. Remaining companies are scored on
// subsequent loads; until then they sort with a neutral score. A forced re-rank
// ignores this cap. 60 ≈ 1 batch ≈ a few seconds per request.
const maxNewPerCall = 60

// ranksMu guards read/modify/write of the ranks cache file.
var ranksMu sync.Mutex

// rankCache maps a normalized company name → importance score (0-100).
type rankCache map[string]int

func normCompany(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// LoadRanks reads the cached company→score map. Missing file → empty map.
func LoadRanks(path string) (rankCache, error) {
	ranksMu.Lock()
	defer ranksMu.Unlock()
	return loadRanksLocked(path)
}

func loadRanksLocked(path string) (rankCache, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rankCache{}, nil
		}
		return nil, err
	}
	var rc rankCache
	if err := json.Unmarshal(b, &rc); err != nil {
		return nil, err
	}
	if rc == nil {
		rc = rankCache{}
	}
	return rc, nil
}

func saveRanksLocked(path string, rc rankCache) error {
	b, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// EnsureRanks makes sure every company in `companies` has a score in the cache.
// Only companies not already cached are sent to OpenAI (batched), so the first
// run scores everything and later runs are near-instant. When force is true the
// cache is ignored and every company is re-scored. If AI is unavailable or a
// batch fails, missing companies get a neutral score (50) so ranking still works
// (just less differentiated). Returns the full cache.
func EnsureRanks(ctx context.Context, path, apiKey, model string, companies []string, force bool) (rankCache, error) {
	ranksMu.Lock()
	defer ranksMu.Unlock()

	rc, err := loadRanksLocked(path)
	if err != nil {
		return nil, err
	}
	if force {
		rc = rankCache{}
	}

	// Which companies still need a score?
	var todo []string
	for _, c := range companies {
		if _, ok := rc[normCompany(c)]; !ok {
			todo = append(todo, c)
		}
	}
	if len(todo) == 0 {
		return rc, nil
	}

	// Without AI there's nothing to score; leave uncached companies unscored so
	// they read as neutral (via lookup default) and get a real score later if a
	// key is added. Avoids blocking and avoids persisting a permanent 50.
	if apiKey == "" {
		return rc, nil
	}

	// Bound the work per (non-forced) request so a huge sheet doesn't hang the
	// HTTP call. A forced re-rank scores everything.
	if !force && len(todo) > maxNewPerCall {
		todo = todo[:maxNewPerCall]
	}

	changed := false
	for start := 0; start < len(todo); start += rankBatchSize {
		end := start + rankBatchSize
		if end > len(todo) {
			end = len(todo)
		}
		batch := todo[start:end]

		scores, err := scoreBatch(ctx, apiKey, model, batch)
		if err != nil {
			// Batch failed (network/timeout/quota) — stop here and persist what we
			// have; the rest retry on the next call. Don't fail the whole request.
			break
		}
		for _, c := range batch {
			key := normCompany(c)
			if s, ok := scores[key]; ok {
				rc[key] = s
			} else {
				rc[key] = 50 // AI answered but omitted this one → neutral, don't retry forever
			}
			changed = true
		}
	}

	if changed {
		if err := saveRanksLocked(path, rc); err != nil {
			return nil, err
		}
	}
	return rc, nil
}

// scoreBatch asks OpenAI to score a batch of companies 0-100 by how prestigious/
// desirable they are as an employer for a fresher software engineer. Returns a
// map of normalized-company → score. Best-effort: any failure returns an error
// and the caller falls back to neutral scores.
func scoreBatch(ctx context.Context, apiKey, model string, companies []string) (map[string]int, error) {
	sys := strings.Join([]string{
		"You rank companies by how attractive they are as an employer for a fresher software engineer in India.",
		"Score each company 0-100: 90-100 = top global tech / FAANG-tier / famous unicorns;",
		"70-89 = well-known product companies, major MNCs, top consultancies;",
		"40-69 = solid mid-size / recognizable companies; 0-39 = small, obscure, or staffing/agency firms.",
		"Judge by the company NAME only. If unsure, give a middling score.",
		`Return STRICT JSON: {"scores": {"<company>": <int>, ...}} using the EXACT company strings provided as keys. No other text.`,
	}, "\n")

	var b strings.Builder
	b.WriteString("Score these companies:\n")
	for _, c := range companies {
		fmt.Fprintf(&b, "- %s\n", c)
	}

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": b.String()},
		},
		"temperature":     0.2,
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAIURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI returned %d", resp.StatusCode)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		return nil, err
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices")
	}

	var parsed struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return nil, err
	}

	out := make(map[string]int, len(parsed.Scores))
	for name, score := range parsed.Scores {
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		out[normCompany(name)] = score
	}
	return out, nil
}

// ApplyRanksAndSort stamps each contact's Rank from the cache and sorts the
// slice by company score (desc), then company name, then person name — so the
// most important companies float to the top with their contacts grouped.
func ApplyRanksAndSort(contacts []Contact, rc rankCache) {
	for i := range contacts {
		if s, ok := rc[normCompany(contacts[i].Company)]; ok {
			contacts[i].Rank = s
		} else {
			contacts[i].Rank = 50 // not yet AI-scored → neutral
		}
	}
	sort.SliceStable(contacts, func(i, j int) bool {
		a, b := contacts[i], contacts[j]
		if a.Rank != b.Rank {
			return a.Rank > b.Rank
		}
		if ca, cb := normCompany(a.Company), normCompany(b.Company); ca != cb {
			return ca < cb
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}
