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
	Role           string `json:"role"`  // optional; overrides profile.TargetRole for this email
	Track          string `json:"track"` // "sd" (default) or "ai" — selects resume/profile/AI flavor
}

// greeting returns "Hi {name}," when a recipient name is given, else "Hi,".
func greeting(recipientName string) string {
	if n := strings.TrimSpace(recipientName); n != "" {
		return "Hi " + n + ","
	}
	return "Hi,"
}

// RenderOptions carries optional AI-generated tweaks. Empty fields fall back to
// the fixed template. Only the subject and the single "company line" are ever
// AI-supplied — the intro, skills, and closing paragraphs are always the
// template's own text, so the user's real content is never altered.
type RenderOptions struct {
	Subject     string // overrides the template subject when non-empty
	CompanyLine string // overrides the template's "why this company" paragraph
}

// Render builds the email. The template owns the structure and the user's real
// content; AI (via opts) may only replace the subject and the company line.
// The style is warm, first-person, and narrative to match the user's voice.
func Render(p *resume.Profile, in ComposeInput, attachmentName string, opts RenderOptions) Rendered {
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
	subject := firstNonEmpty(strings.TrimSpace(opts.Subject), buildSubject(name, role, company))
	paras := buildParagraphs(p, role, company, opts.CompanyLine)
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

// The email is built from this fixed template. Only {Company} and {Role} are
// substituted per send; paragraphs 1, 2 and 4 are never AI-touched. Paragraph 3
// (the "passion / why this company" line) may be replaced by an AI-tailored
// sentence, falling back to templateCompanyPara below.
//
// NOTE: {Company}/{Role} refer to the TARGET employer/role. Past employers named
// in the body (Carousell, Propel) are intentionally literal and never replaced.
const (
	// %s = "I am <Name>" or "I am" (when no name); then %s = role, %s = company.
	// Role-neutral so it fits backend, full-stack, or general SWE applications.
	templateIntroPara = "%s, a 2026 B.Tech graduate in ECE from NIT Allahabad, " +
		"and I am applying for the %s position at %s. Having interned as a software engineer " +
		"at Carousell and Propel, I gained hands-on experience building and optimizing " +
		"software across the stack — from APIs and databases to the services that tie them " +
		"together — and learned how to handle real-world challenges in large-scale production environments."

	templateProjectPara = "In addition to my internships, I led the development of the MNNIT (NIT Allahabad) " +
		"Library Book Allotment System, which is actively used by thousands of students. This project honed " +
		"my skills in system design and problem-solving, and it was rewarding to see the system make a tangible impact."

	templateCompanyPara = "I am passionate about building reliable, well-designed software that solves real " +
		"problems for users at scale. I look forward to the possibility of contributing to " +
		"your team at %s and continuing to grow in a challenging environment."

	templateClosePara = "My resume is attached for more details on my background and skills."

	// signatureTitle is a fixed, role-neutral title shown in the email signature,
	// so it never contradicts the specific role being applied for.
	signatureTitle = "Software Engineer"
)

// DefaultCompanyLine is the fixed, safe "why this company" paragraph used when
// AI is off or its output is unusable.
func DefaultCompanyLine(company string) string {
	return fmt.Sprintf(templateCompanyPara, company)
}

// buildParagraphs assembles the body from the fixed template, substituting the
// target company/role. Only the company paragraph may be AI-supplied (falling
// back to the template's own wording when empty).
func buildParagraphs(p *resume.Profile, role, company, companyLine string) []string {
	// "I am Ankit Raj" when a name exists, else just "I am".
	iam := "I am"
	if n := strings.TrimSpace(p.Name); n != "" {
		iam = "I am " + n
	}

	// 1) Intro (FIXED template; name/role/company substituted).
	intro := fmt.Sprintf(templateIntroPara, iam, role, company)

	// 2) Project paragraph (FIXED).
	project := templateProjectPara

	// 3) Passion / why-this-company paragraph (AI-TWEAKABLE; default is template).
	companyPara := strings.TrimSpace(companyLine)
	if companyPara == "" {
		companyPara = DefaultCompanyLine(company)
	}

	// 4) Close (FIXED).
	return []string{intro, project, companyPara, templateClosePara}
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
	lines = append(lines, signatureTitle)
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
	sb.WriteString(`<span style="color:#555;">` + html.EscapeString(signatureTitle) + `</span><br>`)
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
