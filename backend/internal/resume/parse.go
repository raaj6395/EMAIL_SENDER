package resume

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

var (
	emailRe    = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRe    = regexp.MustCompile(`(\+?\d[\d\s().\-]{7,}\d)`)
	linkedInRe = regexp.MustCompile(`(?i)(https?://)?(www\.)?linkedin\.com/[^\s)]+`)
	githubRe   = regexp.MustCompile(`(?i)(https?://)?(www\.)?github\.com/[^\s)]+`)
	urlRe      = regexp.MustCompile(`(?i)https?://[^\s)]+`)

	// Section headers we care about, matched case-insensitively at line start.
	skillsHeaderRe  = regexp.MustCompile(`(?i)^\s*(technical\s+)?(skills|technologies|tech\s+stack|core\s+competencies)\b`)
	summaryHeaderRe = regexp.MustCompile(`(?i)^\s*(summary|objective|profile|about)\b`)
	// A generic "next section" header — an all-caps or Title-case short line.
	nextSectionRe = regexp.MustCompile(`(?i)^\s*(experience|education|projects|work|employment|certifications|awards|interests|languages|publications|contact|references)\b`)

	// Common job-title keywords used to guess a target role.
	titleRe = regexp.MustCompile(`(?i)\b((senior|junior|lead|staff|principal|associate)\s+)?(software|backend|frontend|full[\s\-]?stack|data|machine\s+learning|ml|devops|cloud|mobile|android|ios|site\s+reliability|platform|security|qa)\s+(engineer|developer|scientist|architect|analyst)\b`)
)

// ExtractText reads the full text content of the PDF at path.
func ExtractText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		// GetPlainText can fail on some PDFs; fall back to page-by-page.
		text, perr := extractPerPage(r)
		if perr != nil {
			return "", err
		}
		return text, nil
	}
	if _, err := io.Copy(&buf, b); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func extractPerPage(r *pdf.Reader) (string, error) {
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		txt, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(txt)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// ParseProfile extracts a best-effort Profile from resume text.
// Heuristics are intentionally forgiving — the user reviews/edits the result
// in the UI before anything is sent.
func ParseProfile(text string) *Profile {
	lines := splitLines(text)
	p := &Profile{}

	p.Email = firstMatch(emailRe, text)
	p.Phone = strings.TrimSpace(firstMatch(phoneRe, text))
	p.LinkedIn = normalizeURL(firstMatch(linkedInRe, text))
	p.GitHub = normalizeURL(firstMatch(githubRe, text))
	p.Portfolio = pickPortfolio(text, p.LinkedIn, p.GitHub, p.Email)

	p.Name = guessName(lines, p.Email)
	p.Skills = extractSkills(lines)
	p.TargetRole = guessTargetRole(text)
	p.Pitch = extractPitch(lines)

	return p
}

func splitLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

func firstMatch(re *regexp.Regexp, s string) string {
	return re.FindString(s)
}

func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(u), "http") {
		u = "https://" + u
	}
	return strings.TrimRight(u, ".,);")
}

// pickPortfolio returns the first URL that isn't the LinkedIn/GitHub/email.
func pickPortfolio(text, linkedin, github, email string) string {
	for _, m := range urlRe.FindAllString(text, -1) {
		m = normalizeURL(m)
		lower := strings.ToLower(m)
		if strings.Contains(lower, "linkedin.com") || strings.Contains(lower, "github.com") {
			continue
		}
		if email != "" && strings.Contains(lower, strings.ToLower(email)) {
			continue
		}
		return m
	}
	return ""
}

// guessName returns the topmost line that looks like a person's name:
// short, mostly letters, not an email/url/section header.
func guessName(lines []string, email string) string {
	for _, l := range lines {
		if l == "" {
			continue
		}
		if emailRe.MatchString(l) || urlRe.MatchString(l) {
			continue
		}
		words := strings.Fields(l)
		if len(words) < 1 || len(words) > 4 {
			continue
		}
		if !looksLikeName(l) {
			continue
		}
		return l
	}
	// Fallback: derive from the email local-part.
	if email != "" {
		local := strings.SplitN(email, "@", 2)[0]
		local = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(local)
		return strings.Title(strings.TrimSpace(local)) //nolint:staticcheck // simple casing is fine here
	}
	return ""
}

func looksLikeName(l string) bool {
	letters, others := 0, 0
	for _, r := range l {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			letters++
		case r == ' ' || r == '.' || r == '\'' || r == '-':
			// allowed
		default:
			others++
		}
	}
	return letters >= 2 && others == 0
}

// extractSkills captures the content under a "Skills"/"Technologies" header
// until the next section header, then splits into individual skills.
func extractSkills(lines []string) []string {
	var collecting bool
	var chunk []string
	for _, l := range lines {
		if skillsHeaderRe.MatchString(l) {
			collecting = true
			// Include any inline content after the header on the same line.
			rest := skillsHeaderRe.ReplaceAllString(l, "")
			rest = strings.TrimLeft(rest, " :-\t")
			if rest != "" {
				chunk = append(chunk, rest)
			}
			continue
		}
		if collecting {
			if l == "" {
				continue
			}
			if nextSectionRe.MatchString(l) || summaryHeaderRe.MatchString(l) {
				break
			}
			chunk = append(chunk, l)
		}
	}
	return dedupeSkills(splitSkills(strings.Join(chunk, "\n")))
}

var skillSplitRe = regexp.MustCompile(`[,;•·|/\n\t]|\s{2,}| - `)

func splitSkills(s string) []string {
	parts := skillSplitRe.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, " -•·:.\t")
		// Drop overly long fragments (likely prose, not a skill) and empties.
		if p == "" || len(p) > 40 || len(strings.Fields(p)) > 5 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func dedupeSkills(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		key := strings.ToLower(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
		if len(out) >= 25 { // cap to keep it sane
			break
		}
	}
	return out
}

// guessTargetRole finds the first job-title-like phrase in the text.
func guessTargetRole(text string) string {
	m := titleRe.FindString(text)
	if m == "" {
		return ""
	}
	return strings.Title(strings.ToLower(strings.Join(strings.Fields(m), " "))) //nolint:staticcheck
}

// extractPitch grabs the summary/objective paragraph if present.
func extractPitch(lines []string) string {
	var collecting bool
	var chunk []string
	for _, l := range lines {
		if summaryHeaderRe.MatchString(l) {
			collecting = true
			rest := summaryHeaderRe.ReplaceAllString(l, "")
			rest = strings.TrimLeft(rest, " :-\t")
			if rest != "" {
				chunk = append(chunk, rest)
			}
			continue
		}
		if collecting {
			if nextSectionRe.MatchString(l) || skillsHeaderRe.MatchString(l) {
				break
			}
			if l != "" {
				chunk = append(chunk, l)
			}
			// Stop after ~3 lines to keep the pitch short.
			if len(chunk) >= 3 {
				break
			}
		}
	}
	pitch := strings.Join(chunk, " ")
	pitch = strings.Join(strings.Fields(pitch), " ") // collapse whitespace
	if len(pitch) > 320 {
		pitch = strings.TrimSpace(pitch[:320]) + "…"
	}
	return pitch
}
