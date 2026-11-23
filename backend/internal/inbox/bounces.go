package inbox

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-imap"
)

// hardBounceMarkers identify a bounce that proves the mailbox does not exist.
// Soft failures (mailbox full, greylisting, our IP blocked) are deliberately
// absent: they say nothing about the address and must not suppress it.
var hardBounceMarkers = []string{
	"address not found", "5.1.1", "5.1.10", "user unknown", "unknown user",
	"no such user", "does not exist", "mailbox unavailable", "invalid recipient",
	"recipient address rejected", "no such recipient", "unrouteable address",
}

// FetchHardBounces returns recipients reported as non-existent in bounce
// notices ("Address not found") received in the last `days` days. Read-only.
func FetchHardBounces(cfg Config, days int) ([]string, error) {
	c, _, err := openInbox(cfg)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	crit := imap.NewSearchCriteria()
	crit.Header.Add("From", "mailer-daemon")
	crit.Since = time.Now().AddDate(0, 0, -days)
	ids, err := c.Search(crit)
	if err != nil {
		return nil, fmt.Errorf("IMAP search failed: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	section := &imap.BodySectionName{Peek: true}
	messages := make(chan *imap.Message, len(ids))
	done := make(chan error, 1)
	go func() { done <- c.Fetch(seqset, []imap.FetchItem{section.FetchItem()}, messages) }()

	var out []string
	for msg := range messages {
		if body := msg.GetBody(section); body != nil {
			raw, _ := io.ReadAll(body)
			out = append(out, parseHardBounce(raw)...)
		}
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("IMAP fetch failed: %w", err)
	}
	return out, nil
}

// parseHardBounce extracts the failed recipients from a raw bounce message,
// or nil when it is a soft failure. Gmail names them in X-Failed-Recipients;
// standard DSNs (RFC 3464) use "Final-Recipient: rfc822; addr" lines.
func parseHardBounce(raw []byte) []string {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	body, _ := io.ReadAll(msg.Body)
	text := strings.ToLower(string(body))
	hard := false
	for _, m := range hardBounceMarkers {
		if strings.Contains(text, m) {
			hard = true
			break
		}
	}
	if !hard {
		return nil
	}

	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.ToLower(strings.Trim(strings.TrimSpace(s), "<>"))
		if strings.Contains(s, "@") && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, a := range strings.Split(msg.Header.Get("X-Failed-Recipients"), ",") {
		add(a)
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(strings.ToLower(line), "final-recipient:") {
			if i := strings.Index(line, ";"); i >= 0 {
				add(line[i+1:])
			}
		}
	}
	return out
}
