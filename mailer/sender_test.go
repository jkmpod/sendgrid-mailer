package mailer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sendgrid/sendgrid-go"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/models"
)

// newTestEmailer creates an Emailer whose SendGrid client points at the given
// test server instead of the real API.
func newTestEmailer(serverURL string, batchSize int) *Emailer {
	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test Sender",
		MaxBatchSize: batchSize,
		RateDelayMS:  0, // no delay in tests
	}
	e := NewEmailer(cfg)

	// Replace the client's base URL with our test server.
	req := sendgrid.GetRequest("SG.test-key", "/v3/mail/send", serverURL)
	req.Method = "POST"
	e.client = &sendgrid.Client{Request: req}

	return e
}

func TestSendBulk(t *testing.T) {
	simpleTemplate := "<p>Hello {{.Name}}</p>"

	tests := []struct {
		name           string
		recipientCount int
		batchSize      int
		handler        http.HandlerFunc
		wantSent       int
		wantFailed     int
		wantErrors     int // number of BatchErrors
	}{
		{
			name:           "all batches succeed",
			recipientCount: 5,
			batchSize:      3,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			},
			wantSent:   5,
			wantFailed: 0,
			wantErrors: 0,
		},
		{
			name:           "one batch fails",
			recipientCount: 6,
			batchSize:      3,
			handler: func() http.HandlerFunc {
				callCount := 0
				return func(w http.ResponseWriter, r *http.Request) {
					callCount++
					if callCount == 2 {
						w.WriteHeader(http.StatusBadRequest)
						w.Write([]byte(`{"errors":[{"message":"bad request"}]}`))
						return
					}
					w.WriteHeader(http.StatusAccepted)
				}
			}(),
			wantSent:   3,
			wantFailed: 3,
			wantErrors: 1,
		},
		{
			name:           "all batches fail",
			recipientCount: 4,
			batchSize:      2,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"errors":[{"message":"server error"}]}`))
			},
			wantSent:   0,
			wantFailed: 4,
			wantErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			e := newTestEmailer(server.URL, tt.batchSize)
			recipients := makeRecipients(tt.recipientCount)

			result, err := e.SendBulk(recipients, "Test Subject", simpleTemplate, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected top-level error: %v", err)
			}

			if result.TotalSent != tt.wantSent {
				t.Errorf("TotalSent = %d, want %d", result.TotalSent, tt.wantSent)
			}
			if result.TotalFailed != tt.wantFailed {
				t.Errorf("TotalFailed = %d, want %d", result.TotalFailed, tt.wantFailed)
			}
			if len(result.BatchErrors) != tt.wantErrors {
				t.Errorf("BatchErrors count = %d, want %d", len(result.BatchErrors), tt.wantErrors)
			}

			// Verify batch errors contain meaningful error messages.
			for _, be := range result.BatchErrors {
				if be.Err == nil {
					t.Errorf("BatchError at index %d has nil Err", be.BatchIndex)
				}
				if !strings.Contains(be.Err.Error(), "SendGrid returned status") {
					t.Errorf("BatchError message = %q, expected it to mention status code", be.Err.Error())
				}
			}
		})
	}
}

func TestSendTest(t *testing.T) {
	firstRecipient := models.EmailRecipient{
		Email:        "original@example.com",
		Name:         "Alice",
		CustomFields: map[string]string{"company": "Acme"},
	}

	tests := []struct {
		name       string
		testEmails []string
		handler    http.HandlerFunc
		wantSent   int
		wantFailed int
		wantTopErr string // if non-empty, expect a top-level error
	}{
		{
			name:       "test emails receive personalised content",
			testEmails: []string{"tester1@x.com", "tester2@x.com"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			},
			wantSent:   2,
			wantFailed: 0,
		},
		{
			name:       "empty testEmails returns error",
			testEmails: []string{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			},
			wantTopErr: "testEmails must not be empty",
		},
		{
			name:       "SendGrid failure records batch error",
			testEmails: []string{"tester@x.com"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"errors":[{"message":"server error"}]}`))
			},
			wantSent:   0,
			wantFailed: 1,
		},
		{
			name:       "only test addresses receive mail — count matches testEmails",
			testEmails: []string{"tester1@x.com", "tester2@x.com", "tester3@x.com"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			},
			wantSent:   3,
			wantFailed: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			e := newTestEmailer(server.URL, 1000)

			result, err := e.SendTest(tt.testEmails, "Hello", "<p>Hi {{.Name}}</p>", firstRecipient, nil, nil, nil)

			if tt.wantTopErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantTopErr)
				}
				if !strings.Contains(err.Error(), tt.wantTopErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantTopErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected top-level error: %v", err)
			}
			if result.TotalSent != tt.wantSent {
				t.Errorf("TotalSent = %d, want %d", result.TotalSent, tt.wantSent)
			}
			if result.TotalFailed != tt.wantFailed {
				t.Errorf("TotalFailed = %d, want %d", result.TotalFailed, tt.wantFailed)
			}
		})
	}
}

func TestSendTest_SubjectPrefix(t *testing.T) {
	// Verify the subject sent to SendGrid is prefixed with "[TEST] ".
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		capturedBody = buf
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	firstRecipient := models.EmailRecipient{
		Email: "original@example.com",
		Name:  "Alice",
	}

	_, err := e.SendTest([]string{"tester@x.com"}, "Welcome", "<p>Hi</p>", firstRecipient, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to parse captured request body: %v", err)
	}

	subject, ok := payload["subject"].(string)
	if !ok {
		t.Fatalf("expected subject in payload, got: %v", payload)
	}
	if !strings.HasPrefix(subject, "[TEST] ") {
		t.Errorf("subject = %q, want prefix '[TEST] '", subject)
	}
}

func TestSendTest_OnlyTestRecipients(t *testing.T) {
	// Verify that only the test email addresses appear in the API request,
	// NOT the original recipient's email.
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		capturedBody = buf
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	firstRecipient := models.EmailRecipient{
		Email:        "original@example.com",
		Name:         "Alice",
		CustomFields: map[string]string{"company": "Acme"},
	}

	_, err := e.SendTest([]string{"tester@x.com"}, "Hi", "<p>Hello</p>", firstRecipient, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bodyStr := string(capturedBody)
	if strings.Contains(bodyStr, "original@example.com") {
		t.Error("request body contains original recipient email — should only have test emails")
	}
	if !strings.Contains(bodyStr, "tester@x.com") {
		t.Error("request body does not contain test email address")
	}
}

func TestSendBulk_CategoriesForwarded(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		capturedBody = buf
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	recipients := makeRecipients(1)

	wantCategories := []string{"newsletter", "spring-2026"}
	_, err := e.SendBulk(recipients, "Test Subject", "<p>Hi {{.Name}}</p>", nil, nil, wantCategories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to parse captured request body: %v", err)
	}

	raw, ok := payload["categories"]
	if !ok {
		t.Fatal("expected 'categories' field in request body, not found")
	}
	cats, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("expected categories to be array, got %T", raw)
	}
	if len(cats) != len(wantCategories) {
		t.Fatalf("len(categories) = %d, want %d", len(cats), len(wantCategories))
	}
	for i, want := range wantCategories {
		if cats[i].(string) != want {
			t.Errorf("categories[%d] = %q, want %q", i, cats[i], want)
		}
	}
}

func TestSendTest_CategoriesForwarded(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		capturedBody = buf
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	firstRecipient := models.EmailRecipient{Email: "original@example.com", Name: "Alice"}

	wantCategories := []string{"test-category"}
	_, err := e.SendTest([]string{"tester@x.com"}, "Hi", "<p>Hello</p>", firstRecipient, nil, nil, wantCategories)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to parse captured request body: %v", err)
	}

	raw, ok := payload["categories"]
	if !ok {
		t.Fatal("expected 'categories' field in request body, not found")
	}
	cats, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("expected categories to be array, got %T", raw)
	}
	if len(cats) != 1 || cats[0].(string) != "test-category" {
		t.Errorf("categories = %v, want [test-category]", cats)
	}
}
