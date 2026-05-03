package main

import (
	"log"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/server"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Log a masked version of the API key so you can verify it loaded correctly.
	// Shows the first 5 characters (e.g. "SG.ab") — enough to spot stray quotes.
	masked := cfg.APIKey
	if len(masked) > 5 {
		masked = masked[:5] + "..."
	}
	log.Printf("config: apiKey=%q fromEmail=%q testMode=%v", masked, cfg.FromEmail, cfg.TestMode)

	srv := server.NewServer(cfg)
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(srv.Start(":" + cfg.Port))
}
