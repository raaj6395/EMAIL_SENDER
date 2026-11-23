package inbox

import (
	"reflect"
	"testing"
)

func TestParseHardBounce(t *testing.T) {
	gmailNotFound := "From: Mail Delivery Subsystem <mailer-daemon@googlemail.com>\r\n" +
		"X-Failed-Recipients: HR@Example.com\r\n" +
		"Subject: Delivery Status Notification (Failure)\r\n\r\n" +
		"Address not found\r\nYour message wasn't delivered to hr@example.com.\r\n" +
		"Reporting-MTA: dns; googlemail.com\r\nFinal-Recipient: rfc822; hr@example.com\r\nStatus: 5.1.1\r\n"
	if got := parseHardBounce([]byte(gmailNotFound)); !reflect.DeepEqual(got, []string{"hr@example.com"}) {
		t.Fatalf("hard bounce: got %v", got)
	}

	mailboxFull := "From: mailer-daemon@googlemail.com\r\n" +
		"X-Failed-Recipients: busy@example.com\r\n\r\n" +
		"The recipient's inbox is full. Status: 4.2.2\r\n"
	if got := parseHardBounce([]byte(mailboxFull)); got != nil {
		t.Fatalf("soft bounce must not suppress: got %v", got)
	}
}
