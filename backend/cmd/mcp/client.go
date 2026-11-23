package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// backend talks to the local email-sender HTTP API. Every tool goes through it.
type backend struct {
	base string
	http *http.Client
}

// A single send can legitimately take ~2.5 minutes: three SMTP attempts at 45s
// plus the 3s/6s backoff between them. The job search and reply check are
// slower still (an Apify run plus one AI call per item).
const callTimeout = 5 * time.Minute

func newBackend() *backend {
	base := os.Getenv("BACKEND_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	return &backend{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: callTimeout},
	}
}

// errorEnvelope is the backend's uniform failure shape: {"ok":false,"error":...}.
type errorEnvelope struct {
	Error string `json:"error"`
}

// call performs one API request. body may be nil. The raw JSON response is
// returned so tools can hand it to the model as-is; out, if non-nil, also gets
// the decoded value for tools that post-process before replying.
func (b *backend) call(ctx context.Context, method, path string, body any, out any) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, b.base+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the email-sender backend at %s — start it with ./run.sh (%v)", b.base, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The backend's error strings are already written for a human, so pass
		// them straight through rather than wrapping them in a status code.
		var env errorEnvelope
		if json.Unmarshal(raw, &env) == nil && env.Error != "" {
			return nil, fmt.Errorf("%s", env.Error)
		}
		return nil, fmt.Errorf("backend returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("could not parse backend response: %w", err)
		}
	}
	return raw, nil
}

// uploadFile posts one local file as multipart/form-data under field "file".
func (b *backend) uploadFile(ctx context.Context, path string, params map[string]string, localPath string) (json.RawMessage, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", localPath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range params {
		if err := mw.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	part, err := mw.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.base+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the email-sender backend at %s — start it with ./run.sh (%v)", b.base, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env errorEnvelope
		if json.Unmarshal(raw, &env) == nil && env.Error != "" {
			return nil, fmt.Errorf("%s", env.Error)
		}
		return nil, fmt.Errorf("backend returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

// trackQuery builds a "?track=" suffix, normalizing to the two values the
// backend accepts.
func trackQuery(track string) string {
	return "?track=" + url.QueryEscape(normalizeTrack(track))
}

func normalizeTrack(t string) string {
	if strings.EqualFold(strings.TrimSpace(t), "ai") {
		return "ai"
	}
	return "sd"
}
