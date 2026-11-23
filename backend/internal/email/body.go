package email

import (
	"html"
	"regexp"
	"strings"
)

// Every message goes out as multipart/alternative with both an HTML and a
// plaintext part (see buildMessage), so a caller that supplies only one of them
// would ship a blank view to clients that prefer the other. These two helpers
// derive the missing half.

var (
	tagRe       = regexp.MustCompile(`(?s)<[^>]*>`)
	blockEndRe  = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|blockquote)\s*>`)
	brRe        = regexp.MustCompile(`(?i)<br\s*/?>`)
	scriptRe    = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	styleRe     = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`)
	blankLineRe = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText renders an HTML body as readable plaintext: scripts and styles are
// dropped, block-level closes and <br> become newlines, remaining tags are
// stripped, and entities are decoded.
func HTMLToText(h string) string {
	s := scriptRe.ReplaceAllString(h, "")
	s = styleRe.ReplaceAllString(s, "")
	s = brRe.ReplaceAllString(s, "\n")
	s = blockEndRe.ReplaceAllString(s, "\n\n")
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	// &nbsp; decodes to U+00A0, which reads as a space but isn't one — normalize
	// it so the plaintext part contains ordinary spaces.
	s = strings.ReplaceAll(s, "\u00a0", " ")

	// Trim trailing spaces each line left behind by the removed markup.
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	s = strings.Join(lines, "\n")
	s = blankLineRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// TextToHTML wraps plaintext in minimal HTML: blank-line-separated blocks become
// paragraphs and single newlines become <br>. Content is escaped.
func TextToHTML(t string) string {
	t = strings.ReplaceAll(t, "\r\n", "\n")
	var b strings.Builder
	b.WriteString(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;font-size:14px;line-height:1.6;color:#1a1a1a;">`)
	for _, para := range strings.Split(t, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		b.WriteString(`<p style="margin:0 0 12px;">`)
		b.WriteString(strings.ReplaceAll(html.EscapeString(para), "\n", "<br>"))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// FillBodies returns a Rendered with both parts populated, deriving whichever
// one is missing. If both are empty it returns them unchanged — the caller
// validates that.
func FillBodies(bodyHTML, bodyText string) (string, string) {
	hasHTML := strings.TrimSpace(bodyHTML) != ""
	hasText := strings.TrimSpace(bodyText) != ""
	switch {
	case hasHTML && !hasText:
		return bodyHTML, HTMLToText(bodyHTML)
	case hasText && !hasHTML:
		return TextToHTML(bodyText), bodyText
	default:
		return bodyHTML, bodyText
	}
}
