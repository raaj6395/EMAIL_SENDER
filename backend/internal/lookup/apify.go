// Package lookup finds a person's email from a LinkedIn profile URL by running
// an Apify actor. It uses Apify's "run-sync-get-dataset-items" endpoint, which
// runs the actor and returns its dataset in one blocking call — no polling.
package lookup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ErrInvalidURL is returned when the input isn't a plausible LinkedIn profile
// URL. Callers can treat it as a client (400) error rather than an upstream one.
var ErrInvalidURL = errors.New("that doesn't look like a LinkedIn profile URL (expected https://linkedin.com/in/...)")

// Config holds the Apify settings. Field names are configurable because
// different actors return the email/name/company under different keys.
type Config struct {
	Token        string
	ActorID      string // e.g. "iron-crawler/linkedin-email-finder-profile"
	EmailField   string // dataset key holding the email (default "email")
	NameField    string // dataset key holding the full name (default "full_name")
	CompanyField string // dataset key holding the company (default "current_company")
}

// Result is the normalized outcome of a lookup.
type Result struct {
	Found      bool           `json:"found"`
	Email      string         `json:"email"`
	Name       string         `json:"name"`
	Company    string         `json:"company"`
	Confidence string         `json:"confidence"` // e.g. "92" or "" if unknown
	Status     string         `json:"status"`     // e.g. "valid" / "risky" / "unknown"
	Raw        map[string]any `json:"-"`          // full first item, for debugging
}

// linkedInRe matches a plausible LinkedIn profile URL (standard or Sales Nav).
var linkedInRe = regexp.MustCompile(`(?i)^https?://([a-z]+\.)?linkedin\.com/(in|sales/(lead|people))/`)

const apifyBase = "https://api.apify.com/v2/acts"

// FindEmail runs the configured Apify actor for one LinkedIn URL and returns a
// normalized Result. A successful run that yields no email returns
// Result{Found:false} with a nil error; only transport/auth/parse problems
// return an error.
func FindEmail(ctx context.Context, cfg Config, linkedInURL string) (Result, error) {
	if cfg.Token == "" {
		return Result{}, fmt.Errorf("Apify token not configured: set APIFY_TOKEN in backend/.env")
	}
	url := strings.TrimSpace(linkedInURL)
	if !linkedInRe.MatchString(url) {
		return Result{}, ErrInvalidURL
	}

	actor := firstNonEmpty(cfg.ActorID, "anchor/linkedin-to-email")
	// Apify accepts the actor id with "~" instead of "/" in the path; internal
	// actor IDs (e.g. "v2BduQ96tuQA3R41k") have no slash and pass through as-is.
	actorPath := strings.ReplaceAll(actor, "/", "~")

	// Different actors expect different input field names/shapes. We send all the
	// common variants at once — actors ignore fields they don't recognize:
	//   • snipercoder/linkedin-email-finder → linkedin: string
	//   • anchor/linkedin-to-email          → startUrls: [{url}]
	//   • vulnv/linkedin-email-finder       → urls: [string]
	//   • iron-crawler/...                  → linkedin_urls: [string] + extract_email
	reqBody := map[string]any{
		"linkedin":      url,
		"startUrls":     []map[string]string{{"url": url}},
		"urls":          []string{url},
		"linkedin_urls": []string{url},
		"linkedin_url":  url,
		"extract_email": true,
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return Result{}, err
	}

	endpoint := fmt.Sprintf("%s/%s/run-sync-get-dataset-items?token=%s", apifyBase, actorPath, cfg.Token)

	// Give generous headroom: the fast email-only actors return in seconds, but
	// profile-scraping actors can take a couple of minutes.
	reqCtx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("Apify request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Result{}, fmt.Errorf("Apify rejected the token (check APIFY_TOKEN)")
	}
	if resp.StatusCode == http.StatusNotFound {
		return Result{}, fmt.Errorf("Apify actor %q not found (check APIFY_ACTOR_ID)", actor)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("Apify returned %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return Result{}, fmt.Errorf("could not parse Apify response: %w", err)
	}
	if len(items) == 0 {
		return Result{Found: false}, nil
	}

	item := items[0]

	// Some actors report failures (quota reached, blocked, etc.) as a dataset
	// item with an error message rather than an HTTP error. Surface those as a
	// real error so the user sees *why* — not a misleading "no email found".
	// snipercoder puts a "No emails found..." message in the 02_First_name field.
	if msg := firstNonEmpty(
		getString(item, "errorMessage"),
		getString(item, "error"),
		getString(item, "02_First_name"),
	); msg != "" {
		// "No email found" is a normal negative result, not an error.
		if strings.Contains(strings.ToLower(msg), "no email") {
			return Result{Found: false}, nil
		}
		// Only treat it as an error if it actually reads like one.
		if strings.Contains(strings.ToLower(msg), "limit") ||
			strings.Contains(strings.ToLower(msg), "error") ||
			strings.Contains(strings.ToLower(msg), "upgrade") ||
			strings.Contains(strings.ToLower(msg), "blocked") {
			return Result{}, fmt.Errorf("Apify actor error: %s", msg)
		}
	}

	emailKey := firstNonEmpty(cfg.EmailField, "email")
	nameKey := firstNonEmpty(cfg.NameField, "full_name")
	companyKey := firstNonEmpty(cfg.CompanyField, "current_company_name")

	email := strings.TrimSpace(firstNonEmpty(
		getString(item, emailKey),
		getString(item, "04_Email"), // snipercoder
	))
	res := Result{
		Email: email,
		Name: strings.TrimSpace(firstNonEmpty(
			getString(item, nameKey),
			getString(item, "01_Name"), // snipercoder
		)),
		// Company: try the configured/default key, then common fallbacks so the
		// same code works across actor variants.
		Company: strings.TrimSpace(firstNonEmpty(
			getString(item, companyKey),
			getString(item, "current_company_name"),
			getString(item, "current_company"),
			getString(item, "company_name"),
			getString(item, "16_Company_name"), // snipercoder
		)),
		// Verification/confidence signals differ per actor. The default actor
		// reports millionverifier_*; older docs used email_confidence/status.
		Confidence: strings.TrimSpace(firstNonEmpty(
			getString(item, "email_confidence"),
			getString(item, "millionverifier_quality"),
		)),
		Status: strings.TrimSpace(firstNonEmpty(
			getString(item, "email_status"),
			getString(item, "millionverifier_result"),
		)),
		Raw:   item,
		Found: email != "" && strings.Contains(email, "@"),
	}
	return res, nil
}

// getString reads a key as a string, tolerating numbers/bools that some actors
// return for confidence-type fields.
func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// Trim a trailing ".0" for whole numbers (e.g. confidence 92).
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case bool:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
