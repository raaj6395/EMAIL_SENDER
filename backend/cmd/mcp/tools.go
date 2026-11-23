package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// trackDesc is repeated across every tool that picks a resume/profile.
const trackDesc = `Which resume track to use: "sd" for the software-development resume (default), "ai" for the AI/ML resume. Each track has its own resume PDF and saved profile.`

// registerTools wires every tool onto the server.
func registerTools(s *server.MCPServer, b *backend) {
	registerSendTools(s, b)
	registerProfileTools(s, b)
	registerBatchTools(s, b)
	registerScheduleTools(s, b)
}

// ---- helpers ----

// jsonResult renders a backend response as the tool's text output.
func jsonResult(raw json.RawMessage) *mcp.CallToolResult {
	var pretty any
	if json.Unmarshal(raw, &pretty) == nil {
		if out, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			return mcp.NewToolResultText(string(out))
		}
	}
	return mcp.NewToolResultText(string(raw))
}

// simpleCall is the shape most tools take: one request, response passed through.
func simpleCall(ctx context.Context, b *backend, method, path string, body any) (*mcp.CallToolResult, error) {
	raw, err := b.call(ctx, method, path, body, nil)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(raw), nil
}

// ---- compose & send ----

func registerSendTools(s *server.MCPServer, b *backend) {
	s.AddTool(mcp.NewTool("health",
		mcp.WithDescription(`Check that the email-sender backend is running and see which features are configured (Gmail credentials, each track's resume, OpenAI, digest, job search, HR sheet, inbox). Call this first if anything else fails — an error here means the backend isn't running.`),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return simpleCall(ctx, b, "GET", "/api/health", nil)
	})

	s.AddTool(mcp.NewTool("send_email",
		mcp.WithDescription(`Send an email you have written yourself, with the resume attached. Subject and body are used verbatim — nothing is rewritten. This is the main sending tool.

The address is checked for bad syntax and a dead domain before sending; the send is refused if it would obviously bounce. Sends from the configured Gmail account, with the sender display name taken from the saved profile.

Supply body_text, body_html, or both — whichever you omit is derived from the other, since every message goes out with both a plaintext and an HTML part. Plain text with blank lines between paragraphs is usually the right choice for outreach.`),
		mcp.WithString("to", mcp.Required(), mcp.Description("Recipient email address.")),
		mcp.WithString("subject", mcp.Required(), mcp.Description("Subject line, used exactly as given.")),
		mcp.WithString("body_text", mcp.Description("Plaintext body. Blank lines separate paragraphs.")),
		mcp.WithString("body_html", mcp.Description("HTML body. Omit to have it generated from body_text.")),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithString("company", mcp.Description("Company name, recorded in the send history. Does not appear in the email.")),
		mcp.WithString("resume_path", mcp.Description("Absolute path to a specific PDF to attach instead of the track's resume. Must be an existing, non-empty .pdf file. Only use when the user explicitly asks for a particular file.")),
		mcp.WithBoolean("no_resume", mcp.DefaultBool(false), mcp.Description("Send with no attachment at all.")),
		mcp.WithDestructiveHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		to, err := r.RequireString("to")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		subject, err := r.RequireString("subject")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		bodyText := r.GetString("body_text", "")
		bodyHTML := r.GetString("body_html", "")
		if strings.TrimSpace(bodyText) == "" && strings.TrimSpace(bodyHTML) == "" {
			return mcp.NewToolResultError("a body is required — set body_text or body_html"), nil
		}
		return simpleCall(ctx, b, "POST", "/api/send-raw", map[string]any{
			"to":         to,
			"subject":    subject,
			"bodyText":   bodyText,
			"bodyHTML":   bodyHTML,
			"track":      normalizeTrack(r.GetString("track", "sd")),
			"company":    r.GetString("company", ""),
			"resumePath": r.GetString("resume_path", ""),
			"noResume":   r.GetBool("no_resume", false),
		})
	})

	s.AddTool(mcp.NewTool("preview_email",
		mcp.WithDescription(`Generate a draft outreach email from the saved resume profile without sending anything. Returns the subject, HTML body, and plaintext body the backend would send, plus whether AI tailored it or it came from the plain template.

Use this to start from the user's established template and voice, then edit the returned text and send it with send_email. Requires a saved profile for the track.`),
		mcp.WithString("recipient_email", mcp.Required(), mcp.Description("Recipient email address — shapes the greeting and is validated.")),
		mcp.WithString("company", mcp.Required(), mcp.Description("Company being applied to.")),
		mcp.WithString("recipient_name", mcp.Description("Recipient's name, for the greeting.")),
		mcp.WithString("role", mcp.Description("Role being applied for.")),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		in, errRes := composeInput(r)
		if errRes != nil {
			return errRes, nil
		}
		return simpleCall(ctx, b, "POST", "/api/preview", in)
	})

	s.AddTool(mcp.NewTool("send_templated_email",
		mcp.WithDescription(`Compose AND send in one step: the backend writes the email from the saved profile (AI-tailored when OpenAI is configured, otherwise the plain template) and sends it with the track's resume attached.

Use send_email instead when you want control over the wording. Use this when the user just says "email this company" and wants their usual template.`),
		mcp.WithString("recipient_email", mcp.Required(), mcp.Description("Recipient email address.")),
		mcp.WithString("company", mcp.Required(), mcp.Description("Company being applied to.")),
		mcp.WithString("recipient_name", mcp.Description("Recipient's name, for the greeting.")),
		mcp.WithString("role", mcp.Description("Role being applied for.")),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithDestructiveHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		in, errRes := composeInput(r)
		if errRes != nil {
			return errRes, nil
		}
		return simpleCall(ctx, b, "POST", "/api/send", in)
	})

	s.AddTool(mcp.NewTool("verify_email",
		mcp.WithDescription(`Check whether an address is worth sending to: validates the syntax and looks up MX records for the domain. Returns valid plus a code ("ok", "bad_syntax", "no_mx", "error"). A DNS failure reports valid so a flaky network doesn't block a real address. The send tools run this automatically — call it directly only to check an address ahead of time.`),
		mcp.WithString("email", mcp.Required(), mcp.Description("Address to check.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		addr, err := r.RequireString("email")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return simpleCall(ctx, b, "POST", "/api/verify-email", map[string]any{"email": addr})
	})

	s.AddTool(mcp.NewTool("send_history",
		mcp.WithDescription(`List emails already sent, newest first, with recipient, company, subject, status ("sent" or "failed"), and timestamp. Failed sends include the error. Check this before emailing someone to avoid a duplicate.`),
		mcp.WithNumber("limit", mcp.DefaultNumber(25), mcp.Description("How many recent entries to return (default 25, max 200).")),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := r.GetInt("limit", 25)
		if limit <= 0 {
			limit = 25
		}
		if limit > 200 {
			limit = 200
		}
		// The history endpoint is unpaginated and returns every send ever made;
		// trim it here so a long history doesn't flood the context.
		var entries []json.RawMessage
		if _, err := b.call(ctx, "GET", "/api/history", nil, &entries); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		total := len(entries)
		if len(entries) > limit {
			entries = entries[:limit]
		}
		out, _ := json.MarshalIndent(map[string]any{
			"total":   total,
			"showing": len(entries),
			"newest":  entries,
		}, "", "  ")
		return mcp.NewToolResultText(string(out)), nil
	})
}

// composeInput builds the shared {recipientEmail, company, ...} body used by
// preview and templated send.
func composeInput(r mcp.CallToolRequest) (map[string]any, *mcp.CallToolResult) {
	recipient, err := r.RequireString("recipient_email")
	if err != nil {
		return nil, mcp.NewToolResultError(err.Error())
	}
	company, err := r.RequireString("company")
	if err != nil {
		return nil, mcp.NewToolResultError(err.Error())
	}
	return map[string]any{
		"recipientEmail": recipient,
		"recipientName":  r.GetString("recipient_name", ""),
		"company":        company,
		"role":           r.GetString("role", ""),
		"track":          normalizeTrack(r.GetString("track", "sd")),
	}, nil
}

// ---- profile & resume ----

func registerProfileTools(s *server.MCPServer, b *backend) {
	s.AddTool(mcp.NewTool("get_profile",
		mcp.WithDescription(`Read the saved profile for a track: name, email, phone, target role, skills, pitch, and links. This is what the email templates are built from, and the name sets the sender display name and the resume attachment filename.`),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return simpleCall(ctx, b, "GET", "/api/profile"+trackQuery(r.GetString("track", "sd")), nil)
	})

	s.AddTool(mcp.NewTool("set_profile",
		mcp.WithDescription(`Save the profile for a track. This replaces the whole profile, so read it with get_profile first and send back the full object with your edits applied — omitted fields are cleared.`),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithString("name", mcp.Description("Full name. Also the sender display name and the basis for the resume filename (\"Ankit Raj\" gives Ankit_Raj_Resume.pdf).")),
		mcp.WithString("email", mcp.Description("Contact email shown in the email body.")),
		mcp.WithString("phone", mcp.Description("Contact phone.")),
		mcp.WithString("targetRole", mcp.Description("Role being targeted, e.g. \"Backend Engineer\".")),
		mcp.WithArray("skills", mcp.WithStringItems(), mcp.Description("Skills list.")),
		mcp.WithString("pitch", mcp.Description("Short personal pitch used in the email body.")),
		mcp.WithString("linkedin", mcp.Description("LinkedIn URL.")),
		mcp.WithString("github", mcp.Description("GitHub URL.")),
		mcp.WithString("portfolio", mcp.Description("Portfolio URL.")),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		body := map[string]any{
			"name":       r.GetString("name", ""),
			"email":      r.GetString("email", ""),
			"phone":      r.GetString("phone", ""),
			"targetRole": r.GetString("targetRole", ""),
			"skills":     r.GetStringSlice("skills", []string{}),
			"pitch":      r.GetString("pitch", ""),
			"linkedin":   r.GetString("linkedin", ""),
			"github":     r.GetString("github", ""),
			"portfolio":  r.GetString("portfolio", ""),
		}
		return simpleCall(ctx, b, "PUT", "/api/profile"+trackQuery(r.GetString("track", "sd")), body)
	})

	s.AddTool(mcp.NewTool("upload_resume",
		mcp.WithDescription(`Replace a track's resume PDF with a local file. The uploaded file becomes the resume attached to every future send on that track ("sd" replaces resume.pdf, "ai" replaces ai_resume.pdf). Must be a real PDF, 10MB max. This overwrites the existing resume for that track.`),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Absolute path to the PDF on this machine.")),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithDestructiveHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := r.RequireString("file_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		track := normalizeTrack(r.GetString("track", "sd"))
		raw, err := b.uploadFile(ctx, "/api/resume/upload", map[string]string{"track": track}, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(raw), nil
	})

	s.AddTool(mcp.NewTool("parse_resume",
		mcp.WithDescription(`Extract a profile from the track's resume PDF already on disk — name, email, phone, skills, and so on — merged over any saved edits. Returns the parsed profile without saving it; pass it to set_profile to keep it. Use after upload_resume to refresh the profile from a new resume.`),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return simpleCall(ctx, b, "POST", "/api/parse-resume"+trackQuery(r.GetString("track", "sd")), nil)
	})
}

// ---- batch queue ----

// batchActions maps the tool's action enum to its endpoint.
var batchActions = map[string]string{
	"pause":           "/api/batch/pause",
	"resume":          "/api/batch/resume",
	"cancel":          "/api/batch/cancel",
	"clear":           "/api/batch/clear",
	"send_now":        "/api/batch/send-now",
	"cancel_send_now": "/api/batch/send-now/cancel",
	"retry_failed":    "/api/batch/retry-failed",
}

func registerBatchTools(s *server.MCPServer, b *backend) {
	s.AddTool(mcp.NewTool("batch_add",
		mcp.WithDescription(`Queue recipients for bulk sending. Each one gets its own email composed from the profile for its company, sent with the track's resume, spaced a few seconds apart during the configured send windows.

Recipients go in as one per line: "email", "email, Company", or "email, Company, Name". When the company is omitted it is guessed from the email domain.

Addresses already in the queue are skipped, including ones already sent — a non-zero "skipped" count is normal, not an error. There is one global queue with one track; adding recipients switches the whole queue to the track you pass.`),
		mcp.WithString("recipients", mcp.Required(), mcp.Description("One recipient per line: \"email\", \"email, Company\", or \"email, Company, Name\".")),
		mcp.WithString("track", mcp.Enum("sd", "ai"), mcp.DefaultString("sd"), mcp.Description(trackDesc)),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := r.RequireString("recipients")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return simpleCall(ctx, b, "POST", "/api/batch", map[string]any{
			"rows":  rows,
			"track": normalizeTrack(r.GetString("track", "sd")),
		})
	})

	s.AddTool(mcp.NewTool("batch_status",
		mcp.WithDescription(`Show the bulk queue: totals sent/failed/remaining, whether it is paused, whether a send window is open now, when the next one opens, and how much of the per-window cap is used. Individual items are truncated — use the counts for the overall picture.`),
		mcp.WithNumber("items", mcp.Description("How many individual queue items to include (default 20, max 100).")),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := r.GetInt("items", 20)
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		return batchSnapshot(ctx, b, "GET", "/api/batch", limit)
	})

	s.AddTool(mcp.NewTool("batch_control",
		mcp.WithDescription(`Control the bulk queue.

- pause: hold everything, nothing sends even inside a window
- resume: let it drain again on schedule
- cancel: drop recipients still waiting, keeping the sent/failed log
- clear: empty the queue entirely, including finished items
- send_now: drain immediately, IGNORING both the send windows and the per-window cap. This starts sending real email right away — only use it when the user clearly asks to send now.
- cancel_send_now: return to the normal schedule
- retry_failed: requeue failed items (they were never delivered, so this cannot duplicate an email)`),
		mcp.WithString("action", mcp.Required(),
			mcp.Enum("pause", "resume", "cancel", "clear", "send_now", "cancel_send_now", "retry_failed"),
			mcp.Description("Which queue operation to perform.")),
		mcp.WithDestructiveHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		action, err := r.RequireString("action")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path, ok := batchActions[action]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown action %q", action)), nil
		}
		return batchSnapshot(ctx, b, "POST", path, 20)
	})
}

// batchSnapshot calls a queue endpoint and truncates the item list. The queue
// is unpaginated, so a large paste would otherwise return every row.
func batchSnapshot(ctx context.Context, b *backend, method, path string, limit int) (*mcp.CallToolResult, error) {
	var payload map[string]any
	if _, err := b.call(ctx, method, path, nil, &payload); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Some endpoints wrap the snapshot under "queue"; others return it bare.
	snap := payload
	if q, ok := payload["queue"].(map[string]any); ok {
		snap = q
	}
	if items, ok := snap["items"].([]any); ok && len(items) > limit {
		snap["items"] = items[:limit]
		snap["itemsTruncated"] = fmt.Sprintf("showing %d of %d items", limit, len(items))
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

// ---- schedule ----

func registerScheduleTools(s *server.MCPServer, b *backend) {
	s.AddTool(mcp.NewTool("get_schedule",
		mcp.WithDescription(`Show the weekly send windows that gate the bulk queue: each window's day and time, the per-window cap, whether a window is open right now, and when the next one opens.

Note that enabled=false means the opposite of what it sounds like: with the schedule off, the queue sends immediately with no window restriction at all.`),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return simpleCall(ctx, b, "GET", "/api/schedule", nil)
	})

	s.AddTool(mcp.NewTool("set_schedule",
		mcp.WithDescription(`Replace the weekly send windows. Windows are given in server local time as day (0=Sunday through 6=Saturday) plus start and end minutes from midnight — 11:00 is 660, 13:00 is 780.

enabled=false does NOT stop sending: it removes the window restriction so the queue drains immediately. To stop sending, use batch_control with pause.

This replaces all windows, so read get_schedule first and send back the full set.`),
		mcp.WithBoolean("enabled", mcp.Required(), mcp.Description("true to restrict sending to the windows below; false to let the queue send at any time.")),
		mcp.WithArray("windows", mcp.Required(),
			mcp.Description("The send windows. Each is {\"day\":0-6,\"startMin\":0-1440,\"endMin\":0-1440} with endMin greater than startMin."),
			mcp.Items(map[string]any{
				"type":     "object",
				"required": []string{"day", "startMin", "endMin"},
				"properties": map[string]any{
					"day":      map[string]any{"type": "integer", "minimum": 0, "maximum": 6, "description": "0=Sunday … 6=Saturday"},
					"startMin": map[string]any{"type": "integer", "minimum": 0, "maximum": 1440, "description": "Start, minutes from midnight (11:00 = 660)"},
					"endMin":   map[string]any{"type": "integer", "minimum": 0, "maximum": 1440, "description": "End, minutes from midnight (13:00 = 780)"},
				},
			}),
		),
		mcp.WithNumber("perWindowCap", mcp.Description("Maximum sends per window. 0 means uncapped.")),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		enabled, err := r.RequireBool("enabled")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		windows, ok := r.GetArguments()["windows"]
		if !ok {
			return mcp.NewToolResultError("windows is required (pass [] together with enabled=false to send at any time)"), nil
		}
		return simpleCall(ctx, b, "PUT", "/api/schedule", map[string]any{
			"enabled":      enabled,
			"windows":      windows,
			"perWindowCap": r.GetInt("perWindowCap", 0),
		})
	})
}
