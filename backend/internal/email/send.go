package email

import (
	"fmt"

	"github.com/wneessen/go-mail"
)

// SMTPConfig holds the credentials and server settings for sending.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
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

// sendNoAttachment delivers a rendered email with no file attachment (digests).
func sendNoAttachment(cfg SMTPConfig, fromName, recipient string, r Rendered) error {
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

func dialAndSend(cfg SMTPConfig, msg *mail.Msg) error {
	client, err := mail.NewClient(cfg.Host,
		mail.WithPort(cfg.Port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithTLSPolicy(mail.TLSMandatory),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(cfg.Password),
	)
	if err != nil {
		return fmt.Errorf("could not create SMTP client: %w", err)
	}
	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send via SMTP (check Gmail App Password and that 2FA is enabled): %w", err)
	}
	return nil
}
