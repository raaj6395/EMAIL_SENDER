// Package httpapi wires the HTTP server: the Server struct, route table,
// middleware, and shared JSON responders. Handlers live in handlers_*.go by
// domain (email, jobs, hr, lookup, profile). main.go just constructs a Server
// and serves it.
package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"emailsender/internal/batch"
	"emailsender/internal/config"
	"emailsender/internal/inbox"
	"emailsender/internal/schedule"
	"emailsender/internal/verify"
)

// Server holds the app's shared dependencies for all handlers.
type Server struct {
	cfg   *config.Config
	batch *batch.Manager
	sched *schedule.Store
}

// New builds a Server from the loaded config and starts the scheduled-send
// worker, which drains the persisted queue during the configured send windows.
func New(cfg *config.Config) *Server {
	sched, err := schedule.LoadStore(cfg.SchedulePath)
	if err != nil {
		// LoadStore still returns a usable store on the default schedule.
		log.Printf("warning: %v", err)
	}
	s := &Server{
		cfg:   cfg,
		sched: sched,
		batch: batch.NewManager(cfg.QueuePath, sched, batchMaxGapSec),
	}
	s.batch.Start(s.sendOneBatch)
	if err := verify.LoadBounced(cfg.BouncedPath); err != nil {
		log.Printf("warning: could not load bounce list: %v", err)
	}
	if cfg.HasInbox() {
		go s.watchBounces()
	}
	return s
}

// watchBounces scans the inbox for hard bounces every 30 minutes and adds the
// dead addresses to the bounce list, which every send path checks.
func (s *Server) watchBounces() {
	for {
		addrs, err := inbox.FetchHardBounces(s.imapConfig(), 30)
		if err != nil {
			log.Printf("bounce scan: %v", err)
		} else if n, err := verify.AddBounced(addrs); err != nil {
			log.Printf("bounce scan: could not save: %v", err)
		} else if n > 0 {
			log.Printf("bounce scan: blocked %d new bounced address(es)", n)
		}
		time.Sleep(30 * time.Minute)
	}
}

// ScheduleSummary describes the active send windows for the startup log, e.g.
// "Mon 11:00–13:00, Wed 11:00–13:00 (cap 40/window)".
func (s *Server) ScheduleSummary() string {
	sc := s.sched.Get()
	if !sc.Enabled {
		return "off — the queue sends immediately"
	}
	if len(sc.Windows) == 0 {
		return "no windows configured — nothing will send"
	}
	labels := make([]string, 0, len(sc.Windows))
	for _, w := range sc.Windows {
		labels = append(labels, w.Label())
	}
	out := strings.Join(labels, ", ")
	if sc.PerWindowCap > 0 {
		out += fmt.Sprintf(" (cap %d/window)", sc.PerWindowCap)
	}
	return out
}

// Shutdown stops the background send worker.
func (s *Server) Shutdown() { s.batch.Stop() }

// Handler returns the fully-wired HTTP handler (routes + middleware).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Profile & resume
	mux.HandleFunc("POST /api/parse-resume", s.handleParseResume)
	mux.HandleFunc("GET /api/profile", s.handleGetProfile)
	mux.HandleFunc("PUT /api/profile", s.handleSaveProfile)

	// Email compose / send / history / digest
	mux.HandleFunc("POST /api/preview", s.handlePreview)
	mux.HandleFunc("POST /api/verify-email", s.handleVerifyEmail)
	mux.HandleFunc("POST /api/exists", s.handleExists)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("POST /api/send-raw", s.handleSendRaw)
	mux.HandleFunc("POST /api/resume/upload", s.handleResumeUpload)
	mux.HandleFunc("GET /api/resume", s.handleResumeGet)
	mux.HandleFunc("GET /api/history", s.handleHistory)

	// Scheduled bulk queue — recipients are added any time and go out only
	// inside the configured send windows.
	mux.HandleFunc("POST /api/batch", s.handleQueueAdd)
	mux.HandleFunc("GET /api/batch", s.handleQueueStatus)
	mux.HandleFunc("POST /api/batch/pause", s.handleQueuePause)
	mux.HandleFunc("POST /api/batch/resume", s.handleQueueResume)
	mux.HandleFunc("POST /api/batch/cancel", s.handleQueueClear)
	mux.HandleFunc("POST /api/batch/clear", s.handleQueueClearAll)
	mux.HandleFunc("POST /api/batch/send-now", s.handleQueueSendNow)
	mux.HandleFunc("POST /api/batch/send-now/cancel", s.handleQueueSendNowCancel)
	mux.HandleFunc("POST /api/batch/retry-failed", s.handleQueueRetryFailed)

	// Send schedule (weekly windows + per-window cap)
	mux.HandleFunc("GET /api/schedule", s.handleGetSchedule)
	mux.HandleFunc("PUT /api/schedule", s.handleSaveSchedule)
	mux.HandleFunc("POST /api/digest", s.handleSendDigest)

	// LinkedIn email lookup
	mux.HandleFunc("POST /api/lookup", s.handleLookup)

	// Job search
	mux.HandleFunc("POST /api/jobs/search", s.handleJobSearch)
	mux.HandleFunc("GET /api/jobs", s.handleJobsList)
	mux.HandleFunc("POST /api/jobs/applied", s.handleMarkApplied)

	// Target companies (who to cold-email, by fresher comp band)
	mux.HandleFunc("GET /api/targets", s.handleTargetsList)
	mux.HandleFunc("POST /api/targets/discover", s.handleTargetsDiscover)
	mux.HandleFunc("POST /api/targets/add", s.handleTargetsAdd)
	mux.HandleFunc("POST /api/targets/remove", s.handleTargetsRemove)

	// HR outreach
	mux.HandleFunc("GET /api/hr/whatsapp", s.handleHRWhatsApp)
	mux.HandleFunc("GET /api/hr/email", s.handleHREmail)
	mux.HandleFunc("POST /api/hr/rerank", s.handleHRRerank)
	mux.HandleFunc("POST /api/hr/sent", s.handleHRMarkSent)

	// Startup outreach shortlist (reach.xlsx → verified + AI-screened contacts)
	mux.HandleFunc("GET /api/reach", s.handleReachList)
	mux.HandleFunc("POST /api/reach/verify", s.handleReachVerify)
	mux.HandleFunc("POST /api/reach/screen", s.handleReachScreen)
	mux.HandleFunc("POST /api/reach/queue", s.handleReachQueue)

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
