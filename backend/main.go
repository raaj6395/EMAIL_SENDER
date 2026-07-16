package main

import (
	"log"
	"net/http"

	"emailsender/internal/config"
	"emailsender/internal/httpapi"
	"emailsender/internal/resume"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Seed a pre-filled profile on first run so the UI isn't blank.
	if existing, _ := resume.LoadProfile(cfg.ProfilePath); existing == nil {
		if err := resume.SaveProfile(cfg.ProfilePath, resume.DefaultProfile()); err != nil {
			log.Printf("warning: could not seed default profile: %v", err)
		} else {
			log.Printf("  seeded default profile at %s", cfg.ProfilePath)
		}
	}

	srv := httpapi.New(cfg)

	addr := ":" + cfg.Port
	log.Printf("email-sender backend listening on %s", addr)
	log.Printf("  data dir:     %s", cfg.DataDir)
	log.Printf("  resume found: %v", cfg.HasResume())
	log.Printf("  gmail creds:  %v", cfg.HasCredentials())
	log.Printf("  openai (ai):  %v (model %s)", cfg.HasAI(), cfg.OpenAIModel)
	log.Printf("  digest to:    %v", cfg.HasDigest())
	log.Printf("  apify lookup: %v (actor %s)", cfg.HasLookup(), cfg.ApifyActorID)
	log.Printf("  jobs search:  %v (actor %s)", cfg.HasJobs(), cfg.JobsActorID)
	log.Printf("  hr outreach:  %v", cfg.HasHR())

	// Surface half-configured features so they're not silently broken.
	for _, msg := range cfg.Warnings() {
		log.Printf("  ⚠ %s", msg)
	}

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
