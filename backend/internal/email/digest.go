package email

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// BuildDigest renders a summary email of the send history.
// Returns a Rendered with no attachment.
func BuildDigest(entries []HistoryEntry) Rendered {
	sent, failed := 0, 0
	for _, e := range entries {
		if e.Status == "sent" {
			sent++
		} else {
			failed++
		}
	}

	subject := fmt.Sprintf("Resume outreach digest — %d sent, %d failed", sent, failed)

	// Plaintext
	var text strings.Builder
	fmt.Fprintf(&text, "Outreach summary: %d total (%d sent, %d failed)\n\n", len(entries), sent, failed)
	if len(entries) == 0 {
		text.WriteString("No emails sent yet.\n")
	}
	for _, e := range entries {
		mark := "✓"
		if e.Status != "sent" {
			mark = "✗"
		}
		fmt.Fprintf(&text, "%s  %s → %s  (%s)\n", mark, e.Company, e.RecipientEmail, formatTime(e.SentAt))
	}

	// HTML
	var htmlB strings.Builder
	htmlB.WriteString(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;font-size:14px;line-height:1.5;color:#1a1a1a;max-width:640px;">`)
	htmlB.WriteString(`<h2 style="font-size:18px;margin:0 0 4px;">Resume outreach digest</h2>`)
	fmt.Fprintf(&htmlB, `<p style="color:#555;margin:0 0 16px;">%d total · <strong style="color:#1a7f37;">%d sent</strong> · <strong style="color:#cf222e;">%d failed</strong></p>`, len(entries), sent, failed)

	if len(entries) == 0 {
		htmlB.WriteString(`<p>No emails sent yet.</p>`)
	} else {
		htmlB.WriteString(`<table style="border-collapse:collapse;width:100%;font-size:13px;">`)
		htmlB.WriteString(`<tr style="text-align:left;border-bottom:2px solid #e4e7eb;"><th style="padding:6px 8px;">Status</th><th style="padding:6px 8px;">Company</th><th style="padding:6px 8px;">Recipient</th><th style="padding:6px 8px;">When</th></tr>`)
		for _, e := range entries {
			badge := `<span style="color:#1a7f37;">✓ sent</span>`
			if e.Status != "sent" {
				badge = `<span style="color:#cf222e;">✗ failed</span>`
			}
			fmt.Fprintf(&htmlB,
				`<tr style="border-bottom:1px solid #eef1f4;"><td style="padding:6px 8px;">%s</td><td style="padding:6px 8px;">%s</td><td style="padding:6px 8px;">%s</td><td style="padding:6px 8px;color:#555;">%s</td></tr>`,
				badge, html.EscapeString(e.Company), html.EscapeString(e.RecipientEmail), html.EscapeString(formatTime(e.SentAt)))
		}
		htmlB.WriteString(`</table>`)
	}
	htmlB.WriteString(`</div>`)

	return Rendered{
		Subject:  subject,
		BodyHTML: htmlB.String(),
		BodyText: text.String(),
	}
}

func formatTime(t time.Time) string {
	return t.Format("Jan 2, 3:04 PM")
}

// SendDigest builds and sends the digest to the given recipient.
func SendDigest(cfg SMTPConfig, fromName, recipient string, entries []HistoryEntry) error {
	rendered := BuildDigest(entries)
	return sendNoAttachment(cfg, fromName, recipient, rendered)
}
