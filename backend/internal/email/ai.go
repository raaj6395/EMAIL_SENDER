package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"emailsender/internal/resume"
)

// AIConfig holds the settings needed to call OpenAI.
type AIConfig struct {
	APIKey string
	Model  string
}

const openAIURL = "https://api.openai.com/v1/chat/completions"

// AITweaks are the small, bounded pieces AI is allowed to produce. Everything
// else in the email is fixed template text.
type AITweaks struct {
	Subject     string `json:"subject"`
	CompanyLine string `json:"companyLine"`
}

// GenerateTweaks asks OpenAI for ONLY a tailored subject line and a single
// "why this company" sentence — not the whole email. This keeps the AI's
// surface tiny so it can't drift or rewrite the user's real content. On any
// failure it returns an error so the caller falls back to the template.
func GenerateTweaks(ctx context.Context, ai AIConfig, p *resume.Profile, in ComposeInput) (AITweaks, error) {
	if ai.APIKey == "" {
		return AITweaks{}, fmt.Errorf("OpenAI key not configured")
	}

	sys, user := buildPrompts(p, in)

	reqBody := map[string]any{
		"model": ai.Model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
		"temperature":     0.6,
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return AITweaks{}, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAIURL, bytes.NewReader(buf))
	if err != nil {
		return AITweaks{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return AITweaks{}, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return AITweaks{}, fmt.Errorf("OpenAI returned %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		return AITweaks{}, fmt.Errorf("could not parse OpenAI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return AITweaks{}, fmt.Errorf("OpenAI returned no choices")
	}

	var parsed AITweaks
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return AITweaks{}, fmt.Errorf("could not parse AI JSON: %w", err)
	}

	parsed.Subject = strings.TrimSpace(parsed.Subject)
	parsed.CompanyLine = sanitizeLine(parsed.CompanyLine)

	// Validate the company line — if it's empty or obviously off, discard it so
	// the template's safe default is used instead.
	if !validCompanyLine(parsed.CompanyLine, in.Company) {
		parsed.CompanyLine = ""
	}
	if parsed.Subject == "" && parsed.CompanyLine == "" {
		return AITweaks{}, fmt.Errorf("AI produced nothing usable")
	}
	return parsed, nil
}

// buildPrompts constructs the system + user prompts. The AI is told to produce
// ONLY the subject and one company-specific sentence.
func buildPrompts(p *resume.Profile, in ComposeInput) (system, user string) {
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = strings.TrimSpace(p.TargetRole)
	}

	system = strings.Join([]string{
		"You help tailor a job-application email that is otherwise written from a fixed template.",
		"You will NOT write the whole email. You produce only TWO things:",
		"1) subject: a short, specific subject line (no clickbait, no emojis, under ~9 words).",
		"2) companyLine: a short paragraph of 1-2 sentences (max ~45 words) expressing the candidate's passion and why they look forward to contributing to THIS specific company, in a warm, sincere, first-person voice.",
		"Rules for companyLine: start with 'I'; mention the company by name; do not invent facts, products, metrics, or claims about the company or candidate; no clichés ('I am writing to', 'I am thrilled/excited'); plain everyday words; it must read naturally as the 3rd paragraph of the email, similar in shape to this default it replaces: " + fmt.Sprintf(templateCompanyPara, in.Company),
		"Match this candidate's natural tone: " + styleSample,
		`Return STRICT JSON: {"subject": string, "companyLine": string}. No other text.`,
	}, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "Company: %s\n", in.Company)
	if role != "" {
		fmt.Fprintf(&b, "Role: %s\n", role)
	}
	if p.Name != "" {
		fmt.Fprintf(&b, "Candidate: %s\n", p.Name)
	}
	if len(p.Skills) > 0 {
		fmt.Fprintf(&b, "Candidate skills: %s\n", strings.Join(topN(p.Skills, 8), ", "))
	}
	b.WriteString("Write the subject and the companyLine paragraph now.")
	return system, b.String()
}

// styleSample anchors the AI to the candidate's own natural, sincere voice.
const styleSample = "I am a final-year B.Tech student who interned as a backend engineer. " +
	"I enjoy building reliable backend applications that handle real users and large amounts of data, " +
	"and I want to keep learning and build things that make a real impact."

// sanitizeLine strips wrapping quotes/whitespace and collapses newlines so a
// single-sentence company line stays a single paragraph.
func sanitizeLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	s = strings.Join(strings.Fields(s), " ") // collapse internal whitespace/newlines
	return strings.TrimSpace(s)
}

// validCompanyLine sanity-checks the AI sentence so a bad one is discarded in
// favour of the template default.
func validCompanyLine(line, company string) bool {
	if line == "" {
		return false
	}
	words := strings.Fields(line)
	if len(words) < 4 || len(words) > 45 { // too short or a runaway paragraph
		return false
	}
	// Must reference the company by name (case-insensitive), so it's actually
	// tailored and not a generic filler.
	if c := strings.TrimSpace(company); c != "" {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(c)) {
			return false
		}
	}
	return true
}

func topN(items []string, n int) []string {
	if len(items) > n {
		return items[:n]
	}
	return items
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
