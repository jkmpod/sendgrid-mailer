package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/server"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv := server.NewServer(cfg)
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(srv.Start(":" + cfg.Port))
}
