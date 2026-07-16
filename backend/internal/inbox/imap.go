// Package inbox reads recent Gmail messages over IMAP (using the same Gmail
// App Password that SMTP sending uses — no extra credential) so the app can
// detect replies to outreach and help draft responses. It is strictly
// read-only: it never deletes, moves, or marks messages.
package inbox

import (
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
)

// Config holds the IMAP connection settings.
type Config struct {
	Host     string // e.g. "imap.gmail.com"
	Port     int    // e.g. 993
	Username string // Gmail address
	Password string // Gmail App Password
}

// Message is a normalized inbox message.
type Message struct {
	FromEmail  string    `json:"fromEmail"`
	FromName   string    `json:"fromName"`
	Subject    string    `json:"subject"`
	Date       time.Time `json:"date"`
	MessageID  string    `json:"messageId"`
	InReplyTo  string    `json:"inReplyTo"`
	References  []string  `json:"references"`
	BodyText   string    `json:"bodyText"`
}

func init() {
	// Tolerate non-UTF-8 charsets in message headers/bodies rather than erroring.
	imap.CharsetReader = charset.Reader
}

// FetchRecent connects to the IMAP server and returns the most recent n messages
// from INBOX (newest first), read-only. n is clamped to a sane range by the
// caller; here we defend with a floor of 1.
func FetchRecent(cfg Config, n int) ([]Message, error) {
	if cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("Gmail credentials not configured (need GMAIL_USER + GMAIL_APP_PASSWORD)")
	}
	if n < 1 {
		n = 1
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %w", addr, err)
	}
	defer c.Logout()

	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("IMAP login failed (check GMAIL_APP_PASSWORD): %w", err)
	}

	// Read-only select so nothing is ever marked/changed.
	mbox, err := c.Select("INBOX", true)
	if err != nil {
		return nil, fmt.Errorf("could not open INBOX: %w", err)
	}
	if mbox.Messages == 0 {
		return []Message{}, nil
	}

	// Fetch the last n messages by sequence number.
	from := uint32(1)
	if mbox.Messages > uint32(n) {
		from = mbox.Messages - uint32(n) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

	section := &imap.BodySectionName{Peek: true} // Peek: don't set the \Seen flag
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchInternalDate, section.FetchItem()}

	messages := make(chan *imap.Message, n)
	done := make(chan error, 1)
	go func() { done <- c.Fetch(seqset, items, messages) }()

	var out []Message
	for msg := range messages {
		out = append(out, normalize(msg, section))
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("IMAP fetch failed: %w", err)
	}

	// Newest first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func normalize(msg *imap.Message, section *imap.BodySectionName) Message {
	m := Message{}
	if msg.Envelope != nil {
		e := msg.Envelope
		m.Subject = decodeHeader(e.Subject)
		m.MessageID = strings.Trim(e.MessageId, "<>")
		m.InReplyTo = strings.Trim(e.InReplyTo, "<>")
		if len(e.From) > 0 {
			m.FromEmail = strings.ToLower(e.From[0].Address())
			m.FromName = decodeHeader(e.From[0].PersonalName)
		}
		if !e.Date.IsZero() {
			m.Date = e.Date
		}
	}
	if m.Date.IsZero() {
		m.Date = msg.InternalDate
	}

	// Parse the body for the plain-text part + References header.
	if body := msg.GetBody(section); body != nil {
		if mr, err := gomail.CreateReader(body); err == nil {
			if refs := mr.Header.Get("References"); refs != "" {
				m.References = splitRefs(refs)
			}
			m.BodyText = readPlainText(mr)
			_ = mr.Close()
		}
	}
	return m
}

// readPlainText walks the MIME parts and returns the first text/plain content
// (falling back to any inline text). HTML-only mails yield a stripped body.
func readPlainText(mr *gomail.Reader) string {
	var htmlFallback string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)
			if strings.HasPrefix(ct, "text/plain") {
				return cleanBody(string(b))
			}
			if strings.HasPrefix(ct, "text/html") && htmlFallback == "" {
				htmlFallback = stripHTML(string(b))
			}
		}
	}
	return cleanBody(htmlFallback)
}

// cleanBody trims quoted-reply history and excess whitespace so the AI sees the
// actual new message, not the whole thread.
func cleanBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		// Stop at common quoted-reply markers.
		if strings.HasPrefix(t, "On ") && strings.HasSuffix(t, "wrote:") {
			break
		}
		if strings.HasPrefix(t, "-----Original Message-----") {
			break
		}
		if strings.HasPrefix(t, ">") {
			continue // quoted line
		}
		kept = append(kept, line)
	}
	out := strings.TrimSpace(strings.Join(kept, "\n"))
	// Collapse 3+ blank lines to one.
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	if len(out) > 4000 {
		out = out[:4000] + "…"
	}
	return out
}

// stripHTML is a minimal tag remover for HTML-only mails.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	if out, err := dec.DecodeHeader(s); err == nil {
		return out
	}
	return s
}

func splitRefs(s string) []string {
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if id := strings.Trim(f, "<>"); id != "" {
			out = append(out, id)
		}
	}
	return out
}
