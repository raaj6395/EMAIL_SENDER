// Command mcp exposes the email-sender backend to Claude over MCP: composing,
// sending with the resume attached, the bulk queue, and the send schedule.
//
// It is a thin client over the local HTTP API — the backend must be running
// (./run.sh). Transport is stdio only and deliberately so: the backend has no
// authentication, so this process is the only thing standing between a caller
// and the ability to send mail from the configured Gmail account.
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

const version = "0.1.0"

func main() {
	s := server.NewMCPServer("email-sender", version,
		server.WithToolCapabilities(true),
		server.WithInstructions(
			"Sends job-application emails from the user's Gmail with their resume attached.\n\n"+
				"Typical flow: preview_email to get a draft in the user's established template and voice, "+
				"edit the wording, then send_email with the final text. Use send_history first to avoid "+
				"emailing the same person twice. For many recipients, batch_add queues them to go out "+
				"gradually during the configured send windows.\n\n"+
				"Sending is real and irreversible — confirm the recipient and the final wording with the "+
				"user before calling send_email, send_templated_email, or batch_control with send_now.",
		),
	)

	registerTools(s, newBackend())

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		os.Exit(1)
	}
}
