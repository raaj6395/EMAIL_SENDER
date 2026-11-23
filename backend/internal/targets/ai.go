package targets

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

// DiscoverOptions shapes what kind of companies a discovery run looks for.
type DiscoverOptions struct {
	MinLPA     float64  // CTC floor for a fresher SDE, in lakhs
	Categories []string // restrict to these categories; empty = all
	Cities     []string // preferred India hiring cities; empty = anywhere
	Count      int      // how many companies to ask for (bounded below)
}

// discovery request bounds — enough to be useful, small enough to stay accurate
// and keep the response under the model's comfortable JSON size.
const (
	minCount     = 5
	maxCount     = 40
	defaultCount = 25
)

func (o DiscoverOptions) normalized() DiscoverOptions {
	if o.Count == 0 {
		o.Count = defaultCount
	}
	if o.Count < minCount {
		o.Count = minCount
	}
	if o.Count > maxCount {
		o.Count = maxCount
	}
	// Never ask the model for companies below the floor — anything under it is
	// filtered out on the way into the cache anyway.
	if o.MinLPA < FloorLPA {
		o.MinLPA = FloorLPA
	}
	return o
}

// aiCompany is the model's JSON shape, kept separate from Company so a schema
// change in the prompt can't silently corrupt the stored model.
type aiCompany struct {
	Name        string   `json:"name"`
	MinLPA      float64  `json:"minLPA"`
	MaxLPA      float64  `json:"maxLPA"`
	Category    string   `json:"category"`
	Confidence  string   `json:"confidence"`
	HQ          string   `json:"hq"`
	Note        string   `json:"note"`
	CareersURL  string   `json:"careersUrl"`
	EmailDomain string   `json:"emailDomain"`
	Tech        []string `json:"tech"`
}

func (a aiCompany) toCompany() Company {
	return Company{
		Name:        strings.TrimSpace(a.Name),
		MinLPA:      a.MinLPA,
		MaxLPA:      a.MaxLPA,
		Category:    normalizeCategory(a.Category),
		Confidence:  normalizeConfidence(a.Confidence),
		HQ:          strings.TrimSpace(a.HQ),
		Note:        strings.TrimSpace(a.Note),
		CareersURL:  strings.TrimSpace(a.CareersURL),
		EmailDomain: strings.ToLower(strings.TrimSpace(a.EmailDomain)),
		Tech:        a.Tech,
	}
}

func normalizeCategory(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case CatHypergrowth, "startup", "unicorn", "soonicorn":
		return CatHypergrowth
	case CatMNC, "big tech", "faang", "multinational":
		return CatMNC
	case CatGCC, "capability centre", "capability center", "r&d":
		return CatGCC
	case CatProduct, "indian product", "saas":
		return CatProduct
	case CatFintech, "finance", "banking":
		return CatFintech
	default:
		return CatOther
	}
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

// sharedRules are the accuracy guardrails both prompts depend on. The model is
// estimating from memory, so the important instruction is to be conservative
// and to flag its own uncertainty rather than invent precise-sounding numbers.
const sharedRules = `Comp figures are TOTAL CTC in INR lakhs per annum (LPA) for a FRESHER (0-1 yr) software engineer in India, base + stock + bonus.
Be realistic and conservative — use the widely-reported band, not the best offer ever made.
Reference points: Google/OpenAI-tier ~45-60, Uber/Atlassian/Rubrik ~30-45, Microsoft/Amazon/Adobe ~25-40,
well-funded unicorns ~18-30, strong Indian product companies ~12-20, typical IT services ~3-7.
Set "confidence" to "high" only when the band is widely documented; "low" when you are unsure.
"category" must be exactly one of: "hypergrowth", "mnc", "gcc", "product", "fintech".
"emailDomain" is the company's primary email domain (e.g. "stripe.com") — no @, no URL.
"careersUrl" must be the real careers page if you are confident, else "".
"note" is ONE short line (max 18 words) on what they build and why a fresher SDE should target them.
"tech" is up to 5 notable technologies they are known for.
Never invent a company that does not exist. Only list companies that actually hire software engineers in India.`

// discoverBatch asks the model to name companies fitting the target profile.
func discoverBatch(ctx context.Context, apiKey, model string, opts DiscoverOptions, exclude []string) ([]Company, error) {
	opts = opts.normalized()

	sys := strings.Join([]string{
		"You are an expert on the Indian tech hiring market and fresher software-engineer compensation.",
		"You name real companies that hire fresher SDEs in India and estimate what they pay.",
		sharedRules,
		`Return STRICT JSON: {"companies": [{"name": string, "minLPA": number, "maxLPA": number, "category": string, "confidence": string, "hq": string, "note": string, "careersUrl": string, "emailDomain": string, "tech": [string]}]}`,
		"No text outside the JSON.",
	}, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "List %d companies that hire FRESHER software engineers in India with a total CTC of at least %.0f LPA.\n",
		opts.Count, opts.MinLPA)

	if len(opts.Categories) > 0 {
		labels := make([]string, 0, len(opts.Categories))
		for _, c := range opts.Categories {
			labels = append(labels, normalizeCategory(c))
		}
		fmt.Fprintf(&b, "Restrict to these categories: %s.\n", strings.Join(labels, ", "))
	} else {
		b.WriteString("Mix hypergrowth startups, product MNCs, GCCs, and strong Indian product companies.\n")
	}

	if len(opts.Cities) > 0 {
		fmt.Fprintf(&b, "Prefer companies hiring in: %s.\n", strings.Join(opts.Cities, ", "))
	}

	// Excluding what we already have is what makes repeated runs productive
	// instead of returning the same famous names every time.
	if len(exclude) > 0 {
		capped := exclude
		if len(capped) > 400 {
			capped = capped[:400]
		}
		fmt.Fprintf(&b, "\nEXCLUDE these companies — they are already on my list, do NOT repeat any of them:\n%s\n",
			strings.Join(capped, ", "))
		b.WriteString("Return only companies NOT in that list. Prefer less-obvious but real ones.\n")
	}

	b.WriteString("\nReturn the JSON now.")

	return callCompanies(ctx, apiKey, model, sys, b.String(), 0.4)
}

// scoreNamed estimates comp for specific companies the user named.
func scoreNamed(ctx context.Context, apiKey, model string, names []string) ([]Company, error) {
	sys := strings.Join([]string{
		"You are an expert on the Indian tech hiring market and fresher software-engineer compensation.",
		"For each company given, estimate what it pays a fresher software engineer in India.",
		sharedRules,
		"If a company does not hire software engineers in India, still return it with your best estimate and confidence \"low\".",
		`Return STRICT JSON: {"companies": [{"name": string, "minLPA": number, "maxLPA": number, "category": string, "confidence": string, "hq": string, "note": string, "careersUrl": string, "emailDomain": string, "tech": [string]}]}`,
		"Use the EXACT company names provided. No text outside the JSON.",
	}, "\n")

	var b strings.Builder
	b.WriteString("Estimate fresher SDE compensation in India for:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	b.WriteString("\nReturn the JSON now.")

	// Low temperature: this is a factual lookup, not a brainstorm.
	return callCompanies(ctx, apiKey, model, sys, b.String(), 0.1)
}

// callCompanies runs one chat completion and parses the {companies:[…]} payload.
func callCompanies(ctx context.Context, apiKey, model, sys, user string, temperature float64) ([]Company, error) {
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
		"temperature":     temperature,
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// Discovery asks for up to 40 companies with several fields each, so allow
	// generous time before giving up.
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
		return nil, fmt.Errorf("OpenAI returned %d: %s", resp.StatusCode, truncate(string(body), 200))
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
		Companies []aiCompany `json:"companies"`
	}
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("could not parse AI JSON: %w", err)
	}

	out := make([]Company, 0, len(parsed.Companies))
	for _, a := range parsed.Companies {
		c := a.toCompany()
		if c.Name == "" {
			continue
		}
		// Guard against nonsense bands (model occasionally flips min/max or
		// returns absolute rupees instead of lakhs).
		if c.MinLPA > c.MaxLPA && c.MaxLPA > 0 {
			c.MinLPA, c.MaxLPA = c.MaxLPA, c.MinLPA
		}
		if c.MinLPA > 200 || c.MaxLPA > 200 {
			continue // clearly not lakhs — drop rather than mislead
		}
		out = append(out, c)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
