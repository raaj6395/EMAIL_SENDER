package inbox

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

// Category is the AI's classification of a reply.
const (
	CatPositive       = "positive"        // interested / next steps
	CatQuestion       = "question"        // asks us something
	CatFollowUpLater  = "follow_up_later" // "reach out again in N weeks"
	CatNotOpen        = "not_open"        // role closed / rejection
	CatOther          = "other"
)

// Verdict is the triage result for one reply.
type Verdict struct {
	Category       string `json:"category"`
	ShouldReply    bool   `json:"shouldReply"`
	FollowUpInDays int    `json:"followUpInDays"`
	Draft          string `json:"draft"`
	Reason         string `json:"reason"`
}

// Classify asks OpenAI to categorize a reply and, when appropriate, draft a
// response in the candidate's voice. On any failure it returns a safe default
// (category "other", shouldReply true, no draft) so a reply is never lost.
func Classify(ctx context.Context, apiKey, model string, p *resume.Profile, m Message) (Verdict, error) {
	safe := Verdict{Category: CatOther, ShouldReply: true, Reason: "Needs your review."}
	if apiKey == "" {
		return safe, fmt.Errorf("OpenAI key not configured")
	}

	sys := strings.Join([]string{
		"You triage replies to a job-seeker's cold outreach emails and draft responses.",
		"Classify the reply into exactly one category:",
		"- \"positive\": interested, wants to talk, moves things forward.",
		"- \"question\": asks the candidate something (availability, details, portfolio).",
		"- \"follow_up_later\": says to check back later / not hiring right now but maybe later.",
		"- \"not_open\": role is closed, not hiring, or a clear rejection.",
		"- \"other\": auto-reply, out-of-office, or unclear.",
		"Rules: not_open => shouldReply=false, no draft. follow_up_later => shouldReply=false and set followUpInDays (21-28). positive/question/other => shouldReply=true with a draft.",
		"The draft is a short, warm, professional reply in first person, in the candidate's voice; reference their interest and that a resume was already shared; do NOT invent facts; no clichés. 2-5 sentences.",
		"reason: one short line explaining the classification.",
		`Return STRICT JSON: {"category":string,"shouldReply":bool,"followUpInDays":int,"draft":string,"reason":string}. No other text.`,
	}, "\n")

	var b strings.Builder
	b.WriteString("CANDIDATE: ")
	if p != nil {
		if p.Name != "" {
			fmt.Fprintf(&b, "%s, ", p.Name)
		}
		role := strings.TrimSpace(p.TargetRole)
		if role == "" {
			role = "Software Engineer"
		}
		fmt.Fprintf(&b, "applying for %s roles.\n", role)
	} else {
		b.WriteString("a software engineering candidate.\n")
	}
	fmt.Fprintf(&b, "\nREPLY FROM: %s <%s>\n", m.FromName, m.FromEmail)
	fmt.Fprintf(&b, "SUBJECT: %s\n", m.Subject)
	fmt.Fprintf(&b, "MESSAGE:\n%s\n\nClassify and (if appropriate) draft a reply now.", truncate(m.BodyText, 2000))

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": b.String()},
		},
		"temperature":     0.4,
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return safe, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAIURL, bytes.NewReader(buf))
	if err != nil {
		return safe, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return safe, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return safe, fmt.Errorf("OpenAI returned %d", resp.StatusCode)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil || len(completion.Choices) == 0 {
		return safe, fmt.Errorf("could not parse OpenAI response")
	}

	var v Verdict
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &v); err != nil {
		return safe, fmt.Errorf("could not parse AI JSON")
	}
	v.Category = normalizeCategory(v.Category)
	v.Draft = strings.TrimSpace(v.Draft)
	v.Reason = strings.TrimSpace(v.Reason)

	// Enforce the rules regardless of what the model returned.
	switch v.Category {
	case CatNotOpen:
		v.ShouldReply = false
		v.Draft = ""
	case CatFollowUpLater:
		v.ShouldReply = false
		if v.FollowUpInDays <= 0 {
			v.FollowUpInDays = 24
		}
	default:
		v.ShouldReply = true
	}
	return v, nil
}

func normalizeCategory(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case CatPositive:
		return CatPositive
	case CatQuestion:
		return CatQuestion
	case CatFollowUpLater, "follow up later", "later":
		return CatFollowUpLater
	case CatNotOpen, "closed", "rejection", "rejected":
		return CatNotOpen
	default:
		return CatOther
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
