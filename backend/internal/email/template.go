package email

import (
	"fmt"
	"html"
	"strings"

	"emailsender/internal/resume"
)

// Rendered is the fully composed email ready to preview or send.
type Rendered struct {
	Subject        string `json:"subject"`
	BodyHTML       string `json:"bodyHTML"`
	BodyText       string `json:"bodyText"`
	AttachmentName string `json:"attachmentName"`
}

// ComposeInput is the per-send information supplied from the UI.
type ComposeInput struct {
	RecipientEmail string `json:"recipientEmail"`
	RecipientName  string `json:"recipientName"` // optional; used in the greeting "Hi {name},"
	Company        string `json:"company"`
	Role           string `json:"role"` // optional; overrides profile.TargetRole for this email
}

// greeting returns "Hi {name}," when a recipient name is given, else "Hi,".
func greeting(recipientName string) string {
	if n := strings.TrimSpace(recipientName); n != "" {
		return "Hi " + n + ","
	}
	return "Hi,"
}

// Render builds a personalized cold email from the profile + per-send input.
// The style is warm, first-person, and narrative (flowing paragraphs, no
// bullets) to match the user's own voice. Kept concise for high reply rates.
func Render(p *resume.Profile, in ComposeInput, attachmentName string) Rendered {
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = strings.TrimSpace(p.TargetRole)
	}
	if role == "" {
		role = "Software Engineer"
	}
	company := strings.TrimSpace(in.Company)
	if company == "" {
		company = "your team"
	}

	name := firstNonEmpty(p.Name, "")
	subject := buildSubject(name, role, company)
	paras := buildParagraphs(p, role, company)
	greet := greeting(in.RecipientName)

	return Rendered{
		Subject:        subject,
		BodyHTML:       buildHTML(p, greet, paras),
		BodyText:       buildPlainText(p, greet, paras),
		AttachmentName: attachmentName,
	}
}

func buildSubject(name, role, company string) string {
	if name == "" {
		return fmt.Sprintf("%s interested in contributing to %s", role, company)
	}
	return fmt.Sprintf("%s — %s interested in %s", name, role, company)
}

// buildParagraphs assembles the body as warm, narrative paragraphs.
func buildParagraphs(p *resume.Profile, role, company string) []string {
	var paras []string

	// 1) Intro: who I am. Prefer the user's own pitch verbatim; else derive.
	intro := strings.TrimSpace(p.Pitch)
	if intro != "" {
		if p.Name != "" {
			intro = fmt.Sprintf("I am %s. %s", p.Name, capitalizeSentence(intro))
		} else {
			intro = capitalizeSentence(intro)
		}
	} else {
		intro = fmt.Sprintf("I am a %s who enjoys building reliable, scalable backend systems.", role)
		if p.Name != "" {
			intro = fmt.Sprintf("I am %s, a %s who enjoys building reliable, scalable backend systems.", p.Name, role)
		}
	}
	paras = append(paras, intro)

	// 2) A light line tying skills to the company, when we have skills.
	if len(p.Skills) > 0 {
		paras = append(paras, fmt.Sprintf(
			"I work across %s, and I'd love to bring that experience to %s and contribute to the products your team is building.",
			joinTop(p.Skills, 5), company))
	} else {
		paras = append(paras, fmt.Sprintf(
			"I'd love to bring my experience to %s and contribute to the products your team is building.", company))
	}

	// 3) Soft close + resume mention.
	paras = append(paras, fmt.Sprintf(
		"I've attached my resume with more detail. I'd welcome the chance to talk about a %s role at %s and how I could contribute.",
		role, company))

	return paras
}

// capitalizeSentence ensures the text starts uppercase and ends with a period.
func capitalizeSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	s = string(r)
	if !strings.HasSuffix(s, ".") && !strings.HasSuffix(s, "!") && !strings.HasSuffix(s, "?") {
		s += "."
	}
	return s
}

func buildPlainText(p *resume.Profile, greet string, paras []string) string {
	var sb strings.Builder
	sb.WriteString(greet)
	sb.WriteString("\n\n")
	sb.WriteString(strings.Join(paras, "\n\n"))
	sb.WriteString("\n\nBest regards,\n")
	sb.WriteString(signatureText(p))
	return sb.String()
}

func buildHTML(p *resume.Profile, greet string, paras []string) string {
	var sb strings.Builder
	sb.WriteString(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;font-size:15px;line-height:1.55;color:#1a1a1a;max-width:560px;">`)
	sb.WriteString(`<p>` + html.EscapeString(greet) + `</p>`)
	for _, para := range paras {
		sb.WriteString(`<p>` + html.EscapeString(para) + `</p>`)
	}
	sb.WriteString(`<p style="margin-top:20px;">Best regards,<br>`)
	sb.WriteString(signatureHTML(p))
	sb.WriteString(`</p></div>`)
	return sb.String()
}

func signatureText(p *resume.Profile) string {
	var lines []string
	if p.Name != "" {
		lines = append(lines, p.Name)
	}
	if p.TargetRole != "" {
		lines = append(lines, p.TargetRole)
	}
	var contacts []string
	if p.Email != "" {
		contacts = append(contacts, p.Email)
	}
	if p.Phone != "" {
		contacts = append(contacts, p.Phone)
	}
	if len(contacts) > 0 {
		lines = append(lines, strings.Join(contacts, " · "))
	}
	for _, l := range []string{p.LinkedIn, p.GitHub, p.Portfolio} {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

func signatureHTML(p *resume.Profile) string {
	var sb strings.Builder
	if p.Name != "" {
		sb.WriteString(`<strong>` + html.EscapeString(p.Name) + `</strong><br>`)
	}
	if p.TargetRole != "" {
		sb.WriteString(`<span style="color:#555;">` + html.EscapeString(p.TargetRole) + `</span><br>`)
	}
	var contacts []string
	if p.Email != "" {
		contacts = append(contacts, fmt.Sprintf(`<a href="mailto:%s" style="color:#0b57d0;">%s</a>`, html.EscapeString(p.Email), html.EscapeString(p.Email)))
	}
	if p.Phone != "" {
		contacts = append(contacts, html.EscapeString(p.Phone))
	}
	if len(contacts) > 0 {
		sb.WriteString(strings.Join(contacts, " &middot; ") + `<br>`)
	}
	var links []string
	if p.LinkedIn != "" {
		links = append(links, link(p.LinkedIn, "LinkedIn"))
	}
	if p.GitHub != "" {
		links = append(links, link(p.GitHub, "GitHub"))
	}
	if p.Portfolio != "" {
		links = append(links, link(p.Portfolio, "Portfolio"))
	}
	if len(links) > 0 {
		sb.WriteString(strings.Join(links, " &middot; "))
	}
	return sb.String()
}

func link(url, label string) string {
	return fmt.Sprintf(`<a href="%s" style="color:#0b57d0;">%s</a>`, html.EscapeString(url), label)
}

func joinTop(items []string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	return strings.Join(items, ", ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
