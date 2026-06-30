package mailer

import (
	"sync"

	"github.com/sendgrid/sendgrid-go"

	"github.com/jkmpod/sendgrid-mailer/config"
)

// EmailerState captures the current runtime configuration and client.
type EmailerState struct {
	FromEmail        string
	FromName         string
	RateDelayMS      int
	TimeoutMS        int
	RetryMaxAttempts int
	RetryBackoffMS   int
	RetryAfterCapMS  int
	Client           *sendgrid.Client
}

// Emailer holds configuration and the SendGrid client needed to send emails.
type Emailer struct {
	// MaxBatchSize is retained for backward compatibility. The app sends one
	// email per recipient and no longer groups recipients into batches, so this
	// field no longer governs batching.
	MaxBatchSize int
	// RateDelayMS is the delay in milliseconds between per-recipient sends.
	RateDelayMS int
	// TimeoutMS is the per-request HTTP timeout in milliseconds for one SendGrid call.
	TimeoutMS int
	// RetryMaxAttempts is the maximum number of send attempts per recipient including the first.
	RetryMaxAttempts int
	// RetryBackoffMS is the base backoff delay in milliseconds used for exponential retry.
	RetryBackoffMS int
	// RetryAfterCapMS caps how long a 429 Retry-After header is honoured, in milliseconds.
	RetryAfterCapMS int
	mu              sync.Mutex // guards fromEmail, fromName, and client
	apiKey          string
	fromEmail       string
	fromName        string
	client          *sendgrid.Client
}

// SetFrom updates the sender address at runtime. Thread-safe.
func (e *Emailer) SetFrom(email, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fromEmail = email
	e.fromName = name
}

// GetState returns the current runtime state of the emailer. Thread-safe.
func (e *Emailer) GetState() EmailerState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return EmailerState{
		FromEmail:        e.fromEmail,
		FromName:         e.fromName,
		RateDelayMS:      e.RateDelayMS,
		TimeoutMS:        e.TimeoutMS,
		RetryMaxAttempts: e.RetryMaxAttempts,
		RetryBackoffMS:   e.RetryBackoffMS,
		RetryAfterCapMS:  e.RetryAfterCapMS,
		Client:           e.client,
	}
}

// SetBaseURL redirects the SendGrid client to the given base URL. Intended
// for tests that point the client at httptest.NewServer; not for production
// use. Thread-safe.
func (e *Emailer) SetBaseURL(baseURL string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	req := sendgrid.GetRequest(e.apiKey, "/v3/mail/send", baseURL)
	e.client = &sendgrid.Client{Request: req}
}

// NewEmailer creates an Emailer from application config. It initialises the
// SendGrid client using the API key from cfg.
func NewEmailer(cfg *config.Config) *Emailer {
	return &Emailer{
		MaxBatchSize:     cfg.MaxBatchSize,
		RateDelayMS:      cfg.RateDelayMS,
		TimeoutMS:        cfg.TimeoutMS,
		RetryMaxAttempts: cfg.RetryMaxAttempts,
		RetryBackoffMS:   cfg.RetryBackoffMS,
		RetryAfterCapMS:  cfg.RetryAfterCapMS,
		apiKey:           cfg.APIKey,
		fromEmail:        cfg.FromEmail,
		fromName:         cfg.FromName,
		client:           sendgrid.NewSendClient(cfg.APIKey),
	}
}
