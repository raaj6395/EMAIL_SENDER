// Package httpapi wires the HTTP server: the Server struct, route table,
// middleware, and shared JSON responders. Handlers live in handlers_*.go by
// domain (email, jobs, hr, lookup, profile). main.go just constructs a Server
// and serves it.
package httpapi

import (
	"encoding/json"
	"net/http"

	"emailsender/internal/batch"
	"emailsender/internal/config"
)

// Server holds the app's shared dependencies for all handlers.
type Server struct {
	cfg   *config.Config
	batch *batch.Manager
}

// New builds a Server from the loaded config.
func New(cfg *config.Config) *Server {
	return &Server{cfg: cfg, batch: batch.NewManager()}
}

// Handler returns the fully-wired HTTP handler (routes + middleware).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Profile & resume
	mux.HandleFunc("POST /api/parse-resume", s.handleParseResume)
	mux.HandleFunc("GET /api/profile", s.handleGetProfile)
	mux.HandleFunc("PUT /api/profile", s.handleSaveProfile)

	// Email compose / send / bulk / history / digest
	mux.HandleFunc("POST /api/preview", s.handlePreview)
	mux.HandleFunc("POST /api/verify-email", s.handleVerifyEmail)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("POST /api/batch", s.handleBatchStart)
	mux.HandleFunc("GET /api/batch", s.handleBatchStatus)
	mux.HandleFunc("POST /api/batch/pause", s.handleBatchPause)
	mux.HandleFunc("POST /api/batch/resume", s.handleBatchResume)
	mux.HandleFunc("POST /api/batch/cancel", s.handleBatchCancel)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/digest", s.handleSendDigest)

	// LinkedIn email lookup
	mux.HandleFunc("POST /api/lookup", s.handleLookup)

	// Job search
	mux.HandleFunc("POST /api/jobs/search", s.handleJobSearch)
	mux.HandleFunc("GET /api/jobs", s.handleJobsList)
	mux.HandleFunc("POST /api/jobs/applied", s.handleMarkApplied)

	// HR outreach
	mux.HandleFunc("GET /api/hr/whatsapp", s.handleHRWhatsApp)
	mux.HandleFunc("GET /api/hr/email", s.handleHREmail)
	mux.HandleFunc("POST /api/hr/rerank", s.handleHRRerank)
	mux.HandleFunc("POST /api/hr/sent", s.handleHRMarkSent)

	// Inbox reply assistant
	mux.HandleFunc("POST /api/replies/check", s.handleRepliesCheck)
	mux.HandleFunc("GET /api/replies", s.handleRepliesList)
	mux.HandleFunc("POST /api/replies/send", s.handleReplySend)
	mux.HandleFunc("POST /api/replies/dismiss", s.handleReplyDismiss)

	// Middleware, outermost first: recover from panics, log requests, then CORS.
	return chain(mux, recoverer, requestLogger, s.corsMiddleware)
}

// ---- shared JSON responders ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}
