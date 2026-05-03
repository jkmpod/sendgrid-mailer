package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration read from environment variables.
type Config struct {
	// APIKey is the SendGrid v3 API key (env: SENDGRID_API_KEY). Required.
	APIKey string
	// FromEmail is the verified sender email address (env: FROM_EMAIL). Required.
	FromEmail string
	// FromName is the sender display name (env: FROM_NAME). Required.
	FromName string
	// MaxBatchSize caps recipients per SendGrid API call (env: MAX_BATCH_SIZE, default 1000).
	MaxBatchSize int
	// RateDelayMS is the inter-batch sleep in milliseconds (env: RATE_DELAY_MS, default 100).
	RateDelayMS int
	// TestMode, when true, diverts every send to TestEmails (env: TEST_MODE, default false).
	TestMode bool
	// TestEmails is the comma-separated test address list (env: TEST_EMAILS). Required when TestMode is true.
	TestEmails []string
	// Port is the HTTP server listen port (env: PORT, default 8080).
	Port string
	// MessagesURL overrides the SendGrid Activity Feed endpoint (env: SENDGRID_MESSAGES_URL, default https://api.sendgrid.com/v3/messages).
	MessagesURL string
	// MaxUploadSizeMB caps the multipart upload body in megabytes (env: MAX_UPLOAD_SIZE_MB, default 10).
	MaxUploadSizeMB int
}

// Load reads configuration from environment variables and returns a populated
// Config pointer. It returns an error if any required variable is missing or
// if an integer variable contains a non-numeric value.
func Load() (*Config, error) {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("SENDGRID_API_KEY environment variable is required")
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		return nil, fmt.Errorf("FROM_EMAIL environment variable is required")
	}

	fromName := os.Getenv("FROM_NAME")
	if fromName == "" {
		return nil, fmt.Errorf("FROM_NAME environment variable is required")
	}

	maxBatchSize := 1000
	if v := strings.TrimSpace(os.Getenv("MAX_BATCH_SIZE")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("MAX_BATCH_SIZE must be a valid integer: %w", err)
		}
		maxBatchSize = n
	}

	rateDelayMS := 100
	if v := strings.TrimSpace(os.Getenv("RATE_DELAY_MS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("RATE_DELAY_MS must be a valid integer: %w", err)
		}
		rateDelayMS = n
	}

	testMode := true
	if v := strings.TrimSpace(os.Getenv("TEST_MODE")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("TEST_MODE must be true or false: %w", err)
		}
		testMode = b
	}

	var testEmails []string
	if v := os.Getenv("TEST_EMAILS"); v != "" {
		for _, email := range strings.Split(v, ",") {
			trimmed := strings.TrimSpace(email)
			if trimmed != "" {
				testEmails = append(testEmails, trimmed)
			}
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	messagesURL := os.Getenv("SENDGRID_MESSAGES_URL")
	if messagesURL == "" {
		messagesURL = "https://api.sendgrid.com/v3/messages"
	}

	maxUploadSizeMB := 10
	if v := strings.TrimSpace(os.Getenv("MAX_UPLOAD_SIZE_MB")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("MAX_UPLOAD_SIZE_MB must be a valid integer: %w", err)
		}
		maxUploadSizeMB = n
	}

	return &Config{
		APIKey:          apiKey,
		FromEmail:       fromEmail,
		FromName:        fromName,
		MaxBatchSize:    maxBatchSize,
		RateDelayMS:     rateDelayMS,
		TestMode:        testMode,
		TestEmails:      testEmails,
		Port:            port,
		MessagesURL:     messagesURL,
		MaxUploadSizeMB: maxUploadSizeMB,
	}, nil
}
