package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
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

// styleSample anchors the AI to the candidate's own natural, sincere voice.
const styleSample = `I am Ankit Raj, a final year B.Tech student in ECE at NIT Allahabad. ` +
	`I have worked as a Backend Engineer Intern where I built backend features, improved system performance, ` +
	`worked on production services, and solved real business problems. I enjoy building reliable backend ` +
	`applications that can handle real users and large amounts of data. ` +
	`I am now looking for a Backend Engineer role where I can keep learning, build scalable systems, ` +
	`and create products that make a real impact.`

// aiEmail is the strict JSON shape we ask the model to return.
type aiEmail struct {
	Subject string   `json:"subject"`
	Body    []string `json:"body"` // paragraphs, plain text
}

// GenerateWithAI asks OpenAI to write a tailored cold email. On any failure
// (missing key, network, bad response) it returns an error so the caller can
// fall back to the template.
func GenerateWithAI(ctx context.Context, ai AIConfig, p *resume.Profile, in ComposeInput, attachmentName string) (Rendered, error) {
	if ai.APIKey == "" {
		return Rendered{}, fmt.Errorf("OpenAI key not configured")
	}

	sys, user := buildPrompts(p, in)

	reqBody := map[string]any{
		"model": ai.Model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return Rendered{}, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, openAIURL, bytes.NewReader(buf))
	if err != nil {
		return Rendered{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Rendered{}, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Rendered{}, fmt.Errorf("OpenAI returned %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		return Rendered{}, fmt.Errorf("could not parse OpenAI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return Rendered{}, fmt.Errorf("OpenAI returned no choices")
	}

	var parsed aiEmail
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return Rendered{}, fmt.Errorf("could not parse AI email JSON: %w", err)
	}
	if strings.TrimSpace(parsed.Subject) == "" || len(parsed.Body) == 0 {
		return Rendered{}, fmt.Errorf("AI email was empty")
	}

	return assembleAIRendered(p, parsed, greeting(in.RecipientName), attachmentName), nil
}

// buildPrompts constructs the system + user prompts from the profile and target.
func buildPrompts(p *resume.Profile, in ComposeInput) (system, user string) {
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = strings.TrimSpace(p.TargetRole)
	}

	system = strings.Join([]string{
		"You help a job seeker write a short, warm, natural job-application email to a company.",
		"Match the candidate's own writing voice shown in the STYLE SAMPLE below: simple, sincere, first-person, flowing paragraphs (no bullet points), plain everyday words. Keep it normal and concise — do NOT make it sound corporate, salesy, or over-polished.",
		"Only lightly adapt the candidate's real background to the specific company and role. Never invent facts, projects, metrics, or company details that were not provided.",
		"Length: 110-170 words. Avoid clichés like 'I am writing to', 'I am thrilled/excited', 'I am reaching out'. No greeting line and no sign-off/signature — those are added separately. Start directly with the first sentence of the body.",
		"",
		"STYLE SAMPLE (the candidate's own tone — mirror this):",
		styleSample,
		"",
		`Return STRICT JSON: {"subject": string, "body": [string, ...]} where body is 3-4 short paragraphs of plain text (no markdown, no bullet characters).`,
	}, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "Company: %s\n", in.Company)
	if role != "" {
		fmt.Fprintf(&b, "Target role: %s\n", role)
	}
	if p.Name != "" {
		fmt.Fprintf(&b, "Candidate name: %s\n", p.Name)
	}
	if p.Pitch != "" {
		fmt.Fprintf(&b, "Candidate summary: %s\n", p.Pitch)
	}
	if len(p.Skills) > 0 {
		fmt.Fprintf(&b, "Key skills: %s\n", strings.Join(p.Skills, ", "))
	}
	b.WriteString("The resume is attached to the email; you may reference that it's attached.")
	user = b.String()
	return system, user
}

// assembleAIRendered turns the AI subject/body into a full Rendered email,
// appending our own consistent greeting + signature (so contact details and
// the recipient's name stay accurate).
func assembleAIRendered(p *resume.Profile, ai aiEmail, greet, attachmentName string) Rendered {
	// Plaintext
	var text strings.Builder
	text.WriteString(greet)
	text.WriteString("\n\n")
	text.WriteString(strings.Join(ai.Body, "\n\n"))
	text.WriteString("\n\nBest regards,\n")
	text.WriteString(signatureText(p))

	// HTML
	var htmlB strings.Builder
	htmlB.WriteString(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;font-size:15px;line-height:1.55;color:#1a1a1a;max-width:560px;">`)
	htmlB.WriteString(`<p>` + html.EscapeString(greet) + `</p>`)
	for _, para := range ai.Body {
		htmlB.WriteString(`<p>` + html.EscapeString(para) + `</p>`)
	}
	htmlB.WriteString(`<p style="margin-top:20px;">Best regards,<br>`)
	htmlB.WriteString(signatureHTML(p))
	htmlB.WriteString(`</p></div>`)

	return Rendered{
		Subject:        strings.TrimSpace(ai.Subject),
		BodyHTML:       htmlB.String(),
		BodyText:       text.String(),
		AttachmentName: attachmentName,
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
