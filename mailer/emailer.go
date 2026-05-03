package mailer

import (
	"sync"

	"github.com/sendgrid/sendgrid-go"

	"github.com/jkmpod/sendgrid-mailer/config"
)

// Emailer holds configuration and the SendGrid client needed to send emails.
type Emailer struct {
	MaxBatchSize int
	RateDelayMS  int
	mu           sync.Mutex // guards fromEmail, fromName
	apiKey       string
	fromEmail    string
	fromName     string
	client       *sendgrid.Client
}

// SetFrom updates the sender address at runtime. Thread-safe.
func (e *Emailer) SetFrom(email, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fromEmail = email
	e.fromName = name
}

// GetFrom returns the current sender address. Thread-safe.
func (e *Emailer) GetFrom() (email, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fromEmail, e.fromName
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
