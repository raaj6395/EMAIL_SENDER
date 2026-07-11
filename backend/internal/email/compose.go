package email

import (
	"context"
	"log"
	"strings"

	"emailsender/internal/resume"
)

// Source indicates how an email body was produced.
type Source string

const (
	SourceAI       Source = "ai"
	SourceTemplate Source = "template"
)

// Result bundles the rendered email with metadata for the UI.
type Result struct {
	Rendered
	Source Source `json:"source"` // "ai" or "template"
	Note   string `json:"note"`   // e.g. why we fell back to the template
}

// Compose produces the email, preferring AI when configured and falling back to
// the deterministic template on any AI failure so sending never breaks.
func Compose(ctx context.Context, ai AIConfig, p *resume.Profile, in ComposeInput, attachmentName string) Result {
	if ai.APIKey != "" {
		rendered, err := GenerateWithAI(ctx, ai, p, in, attachmentName)
		if err == nil {
			return Result{Rendered: rendered, Source: SourceAI}
		}
		// Log server-side; surface a short note to the UI without leaking detail.
		log.Printf("AI generation failed, using template fallback: %v", err)
		tmpl := Render(p, in, attachmentName)
		return Result{Rendered: tmpl, Source: SourceTemplate, Note: "AI unavailable — used template. " + shortReason(err)}
	}
	return Result{Rendered: Render(p, in, attachmentName), Source: SourceTemplate}
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
