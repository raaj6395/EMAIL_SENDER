package jobs

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

const openAIURL = "https://api.openai.com/v1/chat/completions"

// Verdict values returned by the eligibility check.
const (
	VerdictEligible = "eligible"
	VerdictMaybe    = "maybe"
	VerdictNot      = "not"
)

// Eligibility is the AI's read on whether a fresher is a fit for a job.
type Eligibility struct {
	Verdict string `json:"verdict"` // "eligible" | "maybe" | "not"
	Reason  string `json:"reason"`  // one short line, <= 15 words
}

// CheckEligibility asks OpenAI whether a fresher (0-1 yr) candidate is eligible
// for a job, given the candidate's profile and the job's title/seniority/
// requirements. It returns a strict {verdict, reason}. On any failure it
// returns an error so the caller can default to VerdictMaybe — a job is never
// dropped just because the AI call failed.
//
// This lives in the jobs package (taking apiKey/model as plain strings) rather
// than in the email package to avoid an import cycle.
func CheckEligibility(ctx context.Context, apiKey, model string, p *resume.Profile, j Job) (Eligibility, error) {
	if apiKey == "" {
		return Eligibility{}, fmt.Errorf("OpenAI key not configured")
	}

	sys, user := buildEligibilityPrompts(p, j)

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
		"temperature":     0.2,
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return Eligibility{}, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAIURL, bytes.NewReader(buf))
	if err != nil {
		return Eligibility{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Eligibility{}, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Eligibility{}, fmt.Errorf("OpenAI returned %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		return Eligibility{}, fmt.Errorf("could not parse OpenAI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return Eligibility{}, fmt.Errorf("OpenAI returned no choices")
	}

	var parsed Eligibility
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return Eligibility{}, fmt.Errorf("could not parse AI JSON: %w", err)
	}

	parsed.Verdict = normalizeVerdict(parsed.Verdict)
	parsed.Reason = strings.TrimSpace(parsed.Reason)
	if parsed.Verdict == "" {
		return Eligibility{}, fmt.Errorf("AI returned an unrecognized verdict")
	}
	return parsed, nil
}

// buildEligibilityPrompts constructs the system + user prompts. The AI must
// return ONLY {verdict, reason}.
func buildEligibilityPrompts(p *resume.Profile, j Job) (system, user string) {
	system = strings.Join([]string{
		"You screen job postings for a fresher (0-1 years of professional experience) software candidate.",
		"Decide if the candidate is realistically eligible to apply.",
		"Return STRICT JSON: {\"verdict\": string, \"reason\": string}.",
		"verdict must be exactly one of: \"eligible\" (clearly fresher-friendly, <=1 yr or intern/new-grad),",
		"\"maybe\" (borderline, e.g. asks 1-2 yrs or unclear), or \"not\" (clearly needs more experience/senior).",
		"reason: one short line, at most 15 words, plain and specific (e.g. 'Asks 3+ years, too senior').",
		"Do not invent requirements not stated in the posting. No text outside the JSON.",
	}, "\n")

	var b strings.Builder
	b.WriteString("CANDIDATE: fresher software engineer, 0-1 years experience")
	if role := strings.TrimSpace(p.TargetRole); role != "" {
		fmt.Fprintf(&b, ", target role %q", role)
	}
	if len(p.Skills) > 0 {
		fmt.Fprintf(&b, ", skills: %s", strings.Join(topN(p.Skills, 10), ", "))
	}
	b.WriteString(".\n\nJOB POSTING:\n")
	fmt.Fprintf(&b, "Title: %s\n", j.Title)
	if j.Seniority != "" {
		fmt.Fprintf(&b, "Seniority: %s\n", j.Seniority)
	}
	if j.ExperienceLevel != "" {
		fmt.Fprintf(&b, "Experience bucket: %s years\n", j.ExperienceLevel)
	}
	if j.RequirementsSummary != "" {
		fmt.Fprintf(&b, "Requirements: %s\n", truncate(j.RequirementsSummary, 800))
	} else if j.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", truncate(j.Description, 800))
	}
	b.WriteString("\nReturn the verdict and reason now.")
	return system, b.String()
}

// normalizeVerdict coerces the AI's verdict into one of the three known values.
func normalizeVerdict(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case VerdictEligible, "yes", "eligible.":
		return VerdictEligible
	case VerdictMaybe, "borderline", "unclear":
		return VerdictMaybe
	case VerdictNot, "no", "ineligible", "not eligible":
		return VerdictNot
	default:
		return ""
	}
}

func topN(items []string, n int) []string {
	if len(items) > n {
		return items[:n]
	}
	return items
}
