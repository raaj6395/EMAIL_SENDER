package email

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type timeoutErr struct{}

func (timeoutErr) Error() string {
	return "read tcp 192.168.1.11:49332->192.178.211.109:587: i/o timeout"
}
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsTransient(t *testing.T) {
	var _ net.Error = timeoutErr{} // the real i/o timeout implements net.Error

	transient := []error{
		timeoutErr{},
		errors.New("dial failed: read tcp 1.2.3.4:587: i/o timeout"),
		errors.New("send failed: closing SMTP DATA writer: i/o timeout"),
		errors.New("connection reset by peer"),
		errors.New("EOF"),
		errors.New("421 4.7.0 Try again later"),
	}
	for _, err := range transient {
		if !isTransient(err) {
			t.Errorf("isTransient(%q) = false, want true", err)
		}
	}

	permanent := []error{
		errors.New("535 5.7.8 Username and Password not accepted"),
		errors.New("invalid recipient address"),
	}
	for _, err := range permanent {
		if isTransient(err) && !isAuth(err) {
			t.Errorf("isTransient(%q) = true, want false", err)
		}
	}
}

func TestIsAuth(t *testing.T) {
	if !isAuth(errors.New("535 5.7.8 Username and Password not accepted")) {
		t.Error("535 should be recognized as an auth failure")
	}
	if isAuth(timeoutErr{}) {
		t.Error("a network timeout must not be reported as an auth failure")
	}
}

// The old message blamed the App Password for every failure, which is what sent
// the user chasing credentials during a network outage.
func TestDescribeSendErrorDistinguishesCauses(t *testing.T) {
	cfg := SMTPConfig{Host: "smtp.gmail.com", Port: 587}

	netMsg := describeSendError(cfg, timeoutErr{}).Error()
	if !strings.Contains(netMsg, "could not reach Gmail") {
		t.Errorf("timeout message should name the network: %q", netMsg)
	}
	if strings.Contains(strings.ToLower(netMsg), "app password") {
		t.Errorf("timeout message must NOT blame the App Password: %q", netMsg)
	}
	if !strings.Contains(netMsg, "smtp.gmail.com:587") {
		t.Errorf("timeout message should name the host:port actually used: %q", netMsg)
	}

	authMsg := describeSendError(cfg, errors.New("535 Username and Password not accepted")).Error()
	if !strings.Contains(authMsg, "GMAIL_APP_PASSWORD") {
		t.Errorf("auth message should point at the password: %q", authMsg)
	}
}

func TestTimeoutDefault(t *testing.T) {
	if got := (SMTPConfig{}).timeout(); got != defaultTimeout {
		t.Errorf("zero timeout = %v, want the %v default", got, defaultTimeout)
	}
	if got := (SMTPConfig{Timeout: 10 * time.Second}).timeout(); got != 10*time.Second {
		t.Errorf("configured timeout not honored: %v", got)
	}
}
