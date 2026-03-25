package config

import (
	"os"
	"strings"
	"testing"
)

// helper: clear all config-related env vars, then restore them after the test.
func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"SENDGRID_API_KEY", "FROM_EMAIL", "FROM_NAME",
		"MAX_BATCH_SIZE", "RATE_DELAY_MS",
		"TEST_MODE", "TEST_EMAILS",
	}
	for _, k := range keys {
		old, exists := os.LookupEnv(k)
		if exists {
			t.Cleanup(func() { os.Setenv(k, old) })
		} else {
			t.Cleanup(func() { os.Unsetenv(k) })
		}
		os.Unsetenv(k)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name           string
		env            map[string]string
		wantErr        string // substring expected in error message; "" means no error
		wantAPIKey     string
		wantEmail      string
		wantName       string
		wantBatch      int
		wantDelayMS    int
		wantTestMode   bool
		wantTestEmails []string
	}{
		{
			name: "all valid with defaults",
			env: map[string]string{
				"SENDGRID_API_KEY": "SG.test-key",
				"FROM_EMAIL":       "test@example.com",
				"FROM_NAME":        "Test Sender",
			},
			wantAPIKey:  "SG.test-key",
			wantEmail:   "test@example.com",
			wantName:    "Test Sender",
			wantBatch:   1000,
			wantDelayMS: 100,
		},
		{
			name: "all valid with custom integers",
			env: map[string]string{
				"SENDGRID_API_KEY": "SG.key",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
				"MAX_BATCH_SIZE":   "500",
				"RATE_DELAY_MS":    "200",
			},
			wantAPIKey:  "SG.key",
			wantEmail:   "a@b.com",
			wantName:    "A",
			wantBatch:   500,
			wantDelayMS: 200,
		},
		{
			name:    "missing SENDGRID_API_KEY",
			env:     map[string]string{"FROM_EMAIL": "a@b.com", "FROM_NAME": "A"},
			wantErr: "SENDGRID_API_KEY",
		},
		{
			name:    "missing FROM_EMAIL",
			env:     map[string]string{"SENDGRID_API_KEY": "k", "FROM_NAME": "A"},
			wantErr: "FROM_EMAIL",
		},
		{
			name:    "missing FROM_NAME",
			env:     map[string]string{"SENDGRID_API_KEY": "k", "FROM_EMAIL": "a@b.com"},
			wantErr: "FROM_NAME",
		},
		{
			name: "invalid MAX_BATCH_SIZE",
			env: map[string]string{
				"SENDGRID_API_KEY": "k",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
				"MAX_BATCH_SIZE":   "abc",
			},
			wantErr: "MAX_BATCH_SIZE",
		},
		{
			name: "invalid RATE_DELAY_MS",
			env: map[string]string{
				"SENDGRID_API_KEY": "k",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
				"RATE_DELAY_MS":    "xyz",
			},
			wantErr: "RATE_DELAY_MS",
		},
		{
			name: "test mode with emails",
			env: map[string]string{
				"SENDGRID_API_KEY": "SG.key",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
				"TEST_MODE":        "true",
				"TEST_EMAILS":      "test1@x.com, test2@x.com",
			},
			wantAPIKey:     "SG.key",
			wantEmail:      "a@b.com",
			wantName:       "A",
			wantBatch:      1000,
			wantDelayMS:    100,
			wantTestMode:   true,
			wantTestEmails: []string{"test1@x.com", "test2@x.com"},
		},
		{
			name: "test mode true but no emails",
			env: map[string]string{
				"SENDGRID_API_KEY": "k",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
				"TEST_MODE":        "true",
			},
			wantErr: "TEST_EMAILS is required",
		},
		{
			name: "test mode false (default)",
			env: map[string]string{
				"SENDGRID_API_KEY": "SG.key",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
			},
			wantAPIKey:   "SG.key",
			wantEmail:    "a@b.com",
			wantName:     "A",
			wantBatch:    1000,
			wantDelayMS:  100,
			wantTestMode: false,
		},
		{
			name: "invalid TEST_MODE value",
			env: map[string]string{
				"SENDGRID_API_KEY": "k",
				"FROM_EMAIL":       "a@b.com",
				"FROM_NAME":        "A",
				"TEST_MODE":        "maybe",
			},
			wantErr: "TEST_MODE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.APIKey != tt.wantAPIKey {
				t.Errorf("APIKey = %q, want %q", cfg.APIKey, tt.wantAPIKey)
			}
			if cfg.FromEmail != tt.wantEmail {
				t.Errorf("FromEmail = %q, want %q", cfg.FromEmail, tt.wantEmail)
			}
			if cfg.FromName != tt.wantName {
				t.Errorf("FromName = %q, want %q", cfg.FromName, tt.wantName)
			}
			if cfg.MaxBatchSize != tt.wantBatch {
				t.Errorf("MaxBatchSize = %d, want %d", cfg.MaxBatchSize, tt.wantBatch)
			}
			if cfg.RateDelayMS != tt.wantDelayMS {
				t.Errorf("RateDelayMS = %d, want %d", cfg.RateDelayMS, tt.wantDelayMS)
			}
			if cfg.TestMode != tt.wantTestMode {
				t.Errorf("TestMode = %v, want %v", cfg.TestMode, tt.wantTestMode)
			}
			if len(cfg.TestEmails) != len(tt.wantTestEmails) {
				t.Errorf("TestEmails length = %d, want %d", len(cfg.TestEmails), len(tt.wantTestEmails))
			} else {
				for i, email := range cfg.TestEmails {
					if email != tt.wantTestEmails[i] {
						t.Errorf("TestEmails[%d] = %q, want %q", i, email, tt.wantTestEmails[i])
					}
				}
			}
		})
	}
}
