package reach

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"

// screenBatchSize is how many companies go into one AI call. These are mostly
// small startups the model may not know, so batches stay modest to keep each
// judgement attentive rather than skimmed.
const screenBatchSize = 25

// ScreenBatch screens up to `limit` not-yet-screened companies and merges the
// results into the cache. Repeated calls walk through the sheet, so the first
// run costs the most and later ones are near-free. Returns how many companies
// were screened and how many qualified.
func ScreenBatch(ctx context.Context, path, apiKey, model string, companies []string, limit int) (screened, qualified int, err error) {
	if apiKey == "" {
		return 0, 0, fmt.Errorf("OpenAI key not configured: set OPENAI_API_KEY in backend/.env to screen companies")
	}

	mu.Lock()
	c, err := loadLocked(path)
	if err != nil {
		mu.Unlock()
		return 0, 0, err
	}
	var todo []string
	for _, name := range companies {
		if _, ok := c.Screens[NormCompany(name)]; !ok {
			todo = append(todo, name)
			if limit > 0 && len(todo) >= limit {
				break
			}
		}
	}
	mu.Unlock() // don't hold the lock across slow network calls

	if len(todo) == 0 {
		return 0, 0, nil
	}

	var got []Screen
	var lastErr error
	for start := 0; start < len(todo); start += screenBatchSize {
		end := start + screenBatchSize
		if end > len(todo) {
			end = len(todo)
		}
		batch := todo[start:end]

		res, err := screenCall(ctx, apiKey, model, batch)
		if err != nil {
			// Stop and persist what we have; the rest retry on the next call.
			lastErr = err
			break
		}
		got = append(got, res...)
	}

	// Nothing landed at all — surface why rather than reporting a silent no-op.
	if len(got) == 0 && lastErr != nil {
		return 0, 0, lastErr
	}

	mu.Lock()
	defer mu.Unlock()
	c, err = loadLocked(path)
	if err != nil {
		return 0, 0, err
	}
	byName := make(map[string]Screen, len(got))
	for _, s := range got {
		byName[NormCompany(s.Company)] = s
	}
	now := time.Now().Format(time.RFC3339)
	for _, name := range todo {
		key := NormCompany(name)
		s, ok := byName[key]
		if !ok {
			// The model answered but skipped this company. Record a negative so it
			// isn't re-sent on every future run; a re-screen can revisit it.
			s = Screen{Company: name, Confidence: "low", Note: "Not enough information to judge."}
		}
		s.Company = name // keep the sheet's own casing for display
		s.ScreenedAt = now
		c.Screens[key] = s
		screened++
		if s.Qualifies() {
			qualified++
		}
	}
	if err := saveLocked(path, c); err != nil {
		return 0, 0, err
	}
	return screened, qualified, nil
}

// aiScreen is the model's JSON shape, kept separate from Screen so a prompt
// change can't silently corrupt the stored model.
type aiScreen struct {
	Name       string  `json:"name"`
	HiresSDE   bool    `json:"hiresSDE"`
	Fresher    bool    `json:"fresher"`
	MinLPA     float64 `json:"minLPA"`
	MaxLPA     float64 `json:"maxLPA"`
	Confidence string  `json:"confidence"`
	Note       string  `json:"note"`
}

// screenRules are the accuracy guardrails. Most of these companies are small
// Indian startups the model has never heard of, so the dangerous failure mode
// is a confident invented number — hence the explicit instruction to judge
// unknown companies by their profile and mark them low confidence.
const screenRules = `Comp is TOTAL CTC in INR lakhs per annum (LPA) for a FRESHER (0-1 year) software engineer in India: base + stock + bonus.
Be realistic and conservative — use the widely-reported band, not the best offer ever made.
Reference points: Google/OpenAI-tier ~45-60, Uber/Atlassian/Rubrik ~30-45, Microsoft/Amazon/Adobe ~25-40,
well-funded unicorns ~18-30, strong Indian product companies ~12-20, funded early startups ~6-14, typical IT services/staffing ~3-7.
"hiresSDE" is false for non-technical businesses (agencies, staffing, consulting-only, retail, education services) that do not employ software engineers.
"fresher" is false for companies that hire only experienced engineers.
Most companies in this list are small, unknown Indian startups. Do NOT assume they pay well:
a small startup you do not recognise typically pays a fresher 4-10 LPA. Say so rather than guessing high.
Set "confidence" to "high" only for companies whose pay is widely documented, "low" when you are guessing from the name alone.
"note" is ONE short line (max 15 words) on what the company does.
Never invent facts. If you do not recognise a company, estimate from its name/industry and set confidence "low".`

// screenCall runs one chat completion over a batch of companies.
func screenCall(ctx context.Context, apiKey, model string, companies []string) ([]Screen, error) {
	sys := strings.Join([]string{
		"You are an expert on the Indian tech hiring market and fresher software-engineer compensation.",
		"For each company, judge whether it hires FRESHER software engineers in India and what it pays them.",
		screenRules,
		`Return STRICT JSON: {"companies": [{"name": string, "hiresSDE": boolean, "fresher": boolean, "minLPA": number, "maxLPA": number, "confidence": string, "note": string}]}`,
		"Use the EXACT company names provided as \"name\". Return one entry per company. No text outside the JSON.",
	}, "\n")

	var b strings.Builder
	b.WriteString("Judge these companies:\n")
	for _, c := range companies {
		fmt.Fprintf(&b, "- %s\n", c)
	}
	b.WriteString("\nReturn the JSON now.")

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": b.String()},
		},
		"temperature":     0.1, // a factual lookup, not a brainstorm
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAIURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("OpenAI rejected the API key (check OPENAI_API_KEY)")
	}
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
		return nil, fmt.Errorf("could not parse OpenAI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	var parsed struct {
		Companies []aiScreen `json:"companies"`
	}
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("could not parse AI JSON: %w", err)
	}

	out := make([]Screen, 0, len(parsed.Companies))
	for _, a := range parsed.Companies {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		s := Screen{
			Company:    name,
			HiresSDE:   a.HiresSDE,
			Fresher:    a.Fresher,
			MinLPA:     a.MinLPA,
			MaxLPA:     a.MaxLPA,
			Confidence: normalizeConfidence(a.Confidence),
			Note:       strings.TrimSpace(a.Note),
		}
		// Guard against a flipped band or absolute rupees instead of lakhs.
		if s.MinLPA > s.MaxLPA && s.MaxLPA > 0 {
			s.MinLPA, s.MaxLPA = s.MaxLPA, s.MinLPA
		}
		if s.MinLPA > 200 || s.MaxLPA > 200 {
			s.MinLPA, s.MaxLPA = 0, 0 // clearly not lakhs — treat as unknown
			s.Confidence = "low"
		}
		out = append(out, s)
	}
	return out, nil
}

func normalizeConfidence(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
