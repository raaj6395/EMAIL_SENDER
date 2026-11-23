package email

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPConfig holds the credentials and server settings for sending.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	// Timeout bounds a single dial+send attempt. Zero uses defaultTimeout —
	// go-mail's own default (15s) is too tight on a home connection, where each
	// send opens a fresh TLS connection and uploads a resume attachment.
	Timeout time.Duration
}

// Send timing. Gmail over a residential link occasionally drops or stalls a
// connection; a transient failure should not permanently burn a recipient, so
// each send gets a few attempts with a growing pause between them.
const (
	defaultTimeout = 45 * time.Second
	maxAttempts    = 3
	retryBackoff   = 3 * time.Second
)

func (c SMTPConfig) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaultTimeout
}

// isTransient reports whether an error is worth retrying: network timeouts,
// refused/reset connections, and EOFs — as opposed to a rejected password or a
// malformed address, where retrying just wastes time and re-hits Gmail.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, frag := range []string{
		"i/o timeout", "timeout", "connection reset", "connection refused",
		"broken pipe", "eof", "no such host", "network is unreachable",
		"temporary failure", "try again", "421", "450", "451", "452",
	} {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}

// isAuth reports whether the server rejected our credentials, which no amount
// of retrying will fix.
func isAuth(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, frag := range []string{
		"535", "534", "530", "authentication failed", "auth", "credentials",
		"username and password not accepted",
	} {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}

// describeSendError turns a raw SMTP error into something that points at the
// real cause. The old message blamed the App Password for everything, which
// sent users chasing credentials when the actual problem was the network.
func describeSendError(cfg SMTPConfig, err error) error {
	switch {
	case isAuth(err):
		return fmt.Errorf("Gmail rejected the login — check GMAIL_APP_PASSWORD and that 2FA is on: %w", err)
	case isTransient(err):
		return fmt.Errorf("could not reach Gmail after %d attempts (network timeout) — check your connection or whether %s:%d is blocked: %w",
			maxAttempts, cfg.Host, cfg.Port, err)
	default:
		return fmt.Errorf("failed to send via SMTP: %w", err)
	}
}

// Send delivers the rendered email to recipient with the resume attached.
// fromName is the sender's display name (e.g. "Ankit Raj"); if empty, only the
// address is used. Returns a descriptive error suitable for surfacing to the UI.
func Send(cfg SMTPConfig, fromName, recipient string, r Rendered, resumePath string) error {
	msg, err := buildMessage(cfg, fromName, recipient, r)
	if err != nil {
		return err
	}
	msg.AttachFile(resumePath, mail.WithFileName(r.AttachmentName))
	return dialAndSend(cfg, msg)
}

// SendReply sends a plain-text reply in an existing thread. It sets the
// In-Reply-To / References headers (from the original message id) so Gmail and
// other clients thread it under the original conversation. No attachment.
func SendReply(cfg SMTPConfig, fromName, recipient, subject, body, inReplyTo string, references []string) error {
	msg := mail.NewMsg()

	var fromErr error
	if fromName != "" {
		fromErr = msg.FromFormat(fromName, cfg.Username)
	} else {
		fromErr = msg.From(cfg.Username)
	}
	if fromErr != nil {
		return fmt.Errorf("invalid sender address %q: %w", cfg.Username, fromErr)
	}
	if err := msg.To(recipient); err != nil {
		return fmt.Errorf("invalid recipient address %q: %w", recipient, err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	// Threading headers (message ids are angle-bracketed per RFC 5322).
	if inReplyTo != "" {
		ref := "<" + inReplyTo + ">"
		msg.SetGenHeader("In-Reply-To", ref)
		refs := ref
		if len(references) > 0 {
			var b []string
			for _, r := range references {
				if r != "" {
					b = append(b, "<"+r+">")
				}
			}
			b = append(b, ref)
			refs = joinSpace(b)
		}
		msg.SetGenHeader("References", refs)
	}
	return dialAndSend(cfg, msg)
}

func joinSpace(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

// SendNoAttachment delivers a rendered email with no file attachment (digests,
// and raw sends that opt out of the resume).
func SendNoAttachment(cfg SMTPConfig, fromName, recipient string, r Rendered) error {
	msg, err := buildMessage(cfg, fromName, recipient, r)
	if err != nil {
		return err
	}
	return dialAndSend(cfg, msg)
}

// buildMessage constructs the base message (from/to/subject/body) shared by all
// send paths.
func buildMessage(cfg SMTPConfig, fromName, recipient string, r Rendered) (*mail.Msg, error) {
	msg := mail.NewMsg()

	var fromErr error
	if fromName != "" {
		fromErr = msg.FromFormat(fromName, cfg.Username)
	} else {
		fromErr = msg.From(cfg.Username)
	}
	if fromErr != nil {
		return nil, fmt.Errorf("invalid sender address %q: %w", cfg.Username, fromErr)
	}
	if err := msg.To(recipient); err != nil {
		return nil, fmt.Errorf("invalid recipient address %q: %w", recipient, err)
	}

	msg.Subject(r.Subject)
	// HTML as the primary part, plaintext as the alternative for clients that
	// prefer or fall back to text. Order matters: text alternative added after.
	msg.SetBodyString(mail.TypeTextHTML, r.BodyHTML)
	msg.AddAlternativeString(mail.TypeTextPlain, r.BodyText)
	return msg, nil
}

// dialAndSend delivers one message, retrying transient network failures. Each
// attempt uses a fresh client: after a timeout the old connection is in an
// unknown state and reusing it tends to fail again immediately.
func dialAndSend(cfg SMTPConfig, msg *mail.Msg) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		client, err := mail.NewClient(cfg.Host,
			mail.WithPort(cfg.Port),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithTLSPolicy(mail.TLSMandatory),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
			mail.WithTimeout(cfg.timeout()),
		)
		if err != nil {
			return fmt.Errorf("could not create SMTP client: %w", err)
		}

		err = client.DialAndSend(msg)
		if err == nil {
			if attempt > 1 {
				log.Printf("smtp: send succeeded on attempt %d", attempt)
			}
			return nil
		}
		lastErr = err

		// Credentials or a bad address won't improve on a retry.
		if !isTransient(err) {
			break
		}
		if attempt < maxAttempts {
			log.Printf("smtp: attempt %d/%d failed (%v) — retrying in %s", attempt, maxAttempts, err, retryBackoff*time.Duration(attempt))
			time.Sleep(retryBackoff * time.Duration(attempt)) // 3s, then 6s
		}
	}

	return describeSendError(cfg, lastErr)
}
