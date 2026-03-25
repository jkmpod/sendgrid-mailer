package mailer

import (
	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/sendgrid/sendgrid-go"
)

// Emailer holds configuration and the SendGrid client needed to send emails.
type Emailer struct {
	MaxBatchSize int
	RateDelayMS  int
	apiKey       string
	fromEmail    string
	fromName     string
	client       *sendgrid.Client
}

// NewEmailer creates an Emailer from application config. It initialises the
// SendGrid client using the API key from cfg.
func NewEmailer(cfg *config.Config) *Emailer {
	return &Emailer{
		MaxBatchSize: cfg.MaxBatchSize,
		RateDelayMS:  cfg.RateDelayMS,
		apiKey:       cfg.APIKey,
		fromEmail:    cfg.FromEmail,
		fromName:     cfg.FromName,
		client:       sendgrid.NewSendClient(cfg.APIKey),
	}
}
