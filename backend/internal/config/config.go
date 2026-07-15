package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	GmailUser        string
	GmailAppPassword string
	SMTPHost         string
	SMTPPort         int
	Port             string // HTTP port the server listens on
	AllowedOrigin    string // CORS origin for the frontend

	OpenAIKey   string // enables AI-written emails; empty → template-only
	OpenAIModel string // e.g. "gpt-4o"
	DigestTo    string // recipient for the on-demand send digest

	// Apify LinkedIn→email lookup (empty token → feature disabled).
	ApifyToken           string
	ApifyActorID         string
	ApifyFallbackActorID string // tried when the primary actor finds no email
	ApifyEmailField      string
	ApifyNameField       string
	ApifyCompanyField    string

	// LinkedIn job search (reuses ApifyToken; empty token → feature disabled).
	JobsActorID     string
	JobsOpenPath    string
	JobsAppliedPath string
	JobsRunsPath    string // log of actual (paid) Apify runs, for the rate-limit window

	// HR outreach (WhatsApp + email contact sheets). Feature enabled when the
	// xlsx exists on disk.
	HRDataPath      string
	HRRanksPath     string // cached company importance scores
	HRSentPath      string // contacts already reached out to

	DataDir     string // directory holding resume.pdf, profile.json, history.json
	ResumePath  string
	ProfilePath string
	HistoryPath string
}

// Load reads configuration from a .env file (if present) and the environment.
// A missing .env is not fatal — real values may come from the environment.
func Load() (*Config, error) {
	// Best-effort: ignore error if .env doesn't exist.
	_ = godotenv.Load()

	dataDir := getenv("DATA_DIR", "data")
	// Resolve to an absolute path so the server works regardless of CWD.
	absData, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		GmailUser:        os.Getenv("GMAIL_USER"),
		GmailAppPassword: os.Getenv("GMAIL_APP_PASSWORD"),
		SMTPHost:         getenv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:         getenvInt("SMTP_PORT", 587),
		Port:             getenv("PORT", "8080"),
		AllowedOrigin:    getenv("ALLOWED_ORIGIN", "http://localhost:3000"),
		OpenAIKey:        os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:      getenv("OPENAI_MODEL", "gpt-4o"),
		DigestTo:         os.Getenv("DIGEST_TO"),

		ApifyToken:           os.Getenv("APIFY_TOKEN"),
		ApifyActorID:         getenv("APIFY_ACTOR_ID", "snipercoder/linkedin-email-finder"),
		ApifyFallbackActorID: getenv("APIFY_FALLBACK_ACTOR_ID", "vulnv/linkedin-email-finder"),
		ApifyEmailField:      getenv("APIFY_EMAIL_FIELD", "email"),
		ApifyNameField:       getenv("APIFY_NAME_FIELD", "full_name"),
		ApifyCompanyField:    getenv("APIFY_COMPANY_FIELD", "current_company_name"),

		JobsActorID:     getenv("APIFY_JOBS_ACTOR_ID", "vIGxjRrHqDTPuE6M4"),
		JobsOpenPath:    filepath.Join(absData, "jobs_open.json"),
		JobsAppliedPath: filepath.Join(absData, "jobs_applied.json"),
		JobsRunsPath:    filepath.Join(absData, "jobs_runs.json"),

		HRDataPath:  getenv("HR_DATA_PATH", filepath.Join(absData, "HR DATA (1).xlsx")),
		HRRanksPath: filepath.Join(absData, "company_ranks.json"),
		HRSentPath:  filepath.Join(absData, "hr_sent.json"),

		DataDir: absData,
		ResumePath:       filepath.Join(absData, "resume.pdf"),
		ProfilePath:      filepath.Join(absData, "profile.json"),
		HistoryPath:      filepath.Join(absData, "history.json"),
	}

	// Ensure the data directory exists so profile/history writes don't fail.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}

	return cfg, nil
}

// HasCredentials reports whether Gmail SMTP credentials are configured.
func (c *Config) HasCredentials() bool {
	return c.GmailUser != "" && c.GmailAppPassword != ""
}

// HasAI reports whether OpenAI email generation is configured.
func (c *Config) HasAI() bool {
	return c.OpenAIKey != ""
}

// HasDigest reports whether a digest recipient is configured.
func (c *Config) HasDigest() bool {
	return c.DigestTo != ""
}

// HasLookup reports whether Apify LinkedIn→email lookup is configured.
func (c *Config) HasLookup() bool {
	return c.ApifyToken != ""
}

// HasJobs reports whether the LinkedIn job search is configured. It reuses the
// Apify token, so it's on whenever Apify is configured.
func (c *Config) HasJobs() bool {
	return c.ApifyToken != ""
}

// HasHR reports whether the HR outreach spreadsheet is present on disk.
func (c *Config) HasHR() bool {
	info, err := os.Stat(c.HRDataPath)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// HasResume reports whether the resume PDF exists on disk.
func (c *Config) HasResume() bool {
	info, err := os.Stat(c.ResumePath)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// ValidateForSend returns an error describing anything missing that would
// prevent an email from being sent.
func (c *Config) ValidateForSend() error {
	if !c.HasCredentials() {
		return errors.New("Gmail credentials missing: set GMAIL_USER and GMAIL_APP_PASSWORD in backend/.env")
	}
	if !c.HasResume() {
		return errors.New("resume not found: place your resume at backend/data/resume.pdf")
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	return n
}
