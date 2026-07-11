package email

import (
	"context"
	"log"
	"strings"

	"emailsender/internal/resume"
)

// Source indicates how an email was produced.
type Source string

const (
	// SourceAITweaked: the fixed template with AI-tailored subject/company line.
	SourceAITweaked Source = "ai-tweaked"
	// SourceTemplate: the pure template (AI off or its tweaks were unusable).
	SourceTemplate Source = "template"
)

// Result bundles the rendered email with metadata for the UI.
type Result struct {
	Rendered
	Source Source `json:"source"` // "ai-tweaked" or "template"
	Note   string `json:"note"`   // e.g. why we fell back to the pure template
}

// Compose always renders from the fixed template. When AI is configured it asks
// for small tweaks (a tailored subject + one "why this company" sentence) and
// injects them; the rest of the email — the user's real intro, skills, and
// close — is never AI-touched. Any AI failure silently falls back to the pure
// template so sending never breaks.
func Compose(ctx context.Context, ai AIConfig, p *resume.Profile, in ComposeInput, attachmentName string) Result {
	if ai.APIKey == "" {
		return Result{Rendered: Render(p, in, attachmentName, RenderOptions{}), Source: SourceTemplate}
	}

	tweaks, err := GenerateTweaks(ctx, ai, p, in)
	if err != nil {
		log.Printf("AI tweaks failed, using pure template: %v", err)
		return Result{
			Rendered: Render(p, in, attachmentName, RenderOptions{}),
			Source:   SourceTemplate,
			Note:     "AI unavailable — used template. " + shortReason(err),
		}
	}

	rendered := Render(p, in, attachmentName, RenderOptions{
		Subject:     tweaks.Subject,
		CompanyLine: tweaks.CompanyLine,
	})
	// If the company line was discarded by validation, say so (subject may still
	// be AI-tailored).
	note := ""
	if tweaks.CompanyLine == "" {
		note = "AI subject applied; used the standard company line."
	}
	return Result{Rendered: rendered, Source: SourceAITweaked, Note: note}
}

// shortReason gives a user-safe one-liner about why AI failed.
func shortReason(err error) string {
	msg := err.Error()
	switch {
	case containsAny(msg, "401", "invalid_api_key", "Incorrect API key"):
		return "Check your OpenAI API key."
	case containsAny(msg, "429", "quota", "rate limit", "insufficient_quota"):
		return "OpenAI rate limit or quota reached."
	case containsAny(msg, "timeout", "deadline", "context deadline"):
		return "OpenAI timed out."
	default:
		return "See backend logs for details."
	}
}

func containsAny(s string, subs ...string) bool {
	ls := strings.ToLower(s)
	for _, sub := range subs {
		if sub != "" && strings.Contains(ls, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
