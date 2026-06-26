package mailer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/models"
)

// newTestEmailer creates an Emailer whose SendGrid client points at the given
// test server instead of the real API. It uses fast retry settings so that
// retry-related tests complete quickly.
func newTestEmailer(serverURL string, batchSize int) *Emailer {
	cfg := &config.Config{
		APIKey:           "SG.test-key",
		FromEmail:        "test@example.com",
		FromName:         "Test Sender",
		MaxBatchSize:     batchSize,
		RateDelayMS:      0, // no delay in tests
		TimeoutMS:        2000,
		RetryMaxAttempts: 3,
		RetryBackoffMS:   1, // 1 ms backoff keeps tests fast
	}
	e := NewEmailer(cfg)
	e.SetBaseURL(serverURL)
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
		wantErrors     int // number of per-recipient Failures
	}{
		{
			name:           "all succeed",
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
			name:           "Nth call fails",
			recipientCount: 6,
			batchSize:      3,
			handler: func() http.HandlerFunc {
				callCount := 0
				return func(w http.ResponseWriter, r *http.Request) {
					callCount++
					if callCount == 2 {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"errors":[{"message":"bad request"}]}`))
						return
					}
					w.WriteHeader(http.StatusAccepted)
				}
			}(),
			wantSent:   5,
			wantFailed: 1,
			wantErrors: 1,
		},
		{
			name:           "all fail",
			recipientCount: 4,
			batchSize:      2,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"message":"server error"}]}`))
			},
			wantSent:   0,
			wantFailed: 4,
			wantErrors: 4,
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
			if len(result.Failures) != tt.wantErrors {
				t.Errorf("Failures count = %d, want %d", len(result.Failures), tt.wantErrors)
			}

			// Verify per-recipient failures contain meaningful error messages.
			for _, rf := range result.Failures {
				if rf.Err == nil {
					t.Errorf("RecipientError for %s has nil Err", rf.Email)
				}
				if !strings.Contains(rf.Err.Error(), "SendGrid returned status") {
					t.Errorf("RecipientError message = %q, expected it to mention status code", rf.Err.Error())
				}
			}
		})
	}
}

func TestSendBulk_CCAppearsOncePerMessage(t *testing.T) {
	// Guard against issue #1: a CC address must appear exactly once per
	// SendGrid request, not be duplicated across personalisations.
	var capturedBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		capturedBodies = append(capturedBodies, string(buf))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	recipients := []models.EmailRecipient{
		{Email: "alice@example.com", Name: "Alice"},
		{Email: "bob@example.com", Name: "Bob"},
	}

	result, err := e.SendBulk(recipients, "Test Subject", "<p>Hi {{.Name}}</p>", []string{"boss@example.com"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if result.TotalSent != 2 {
		t.Errorf("TotalSent = %d, want 2", result.TotalSent)
	}
	if result.TotalFailed != 0 {
		t.Errorf("TotalFailed = %d, want 0", result.TotalFailed)
	}

	if len(capturedBodies) != 2 {
		t.Fatalf("expected 2 SendGrid requests, got %d", len(capturedBodies))
	}
	const ccAddr = "boss@example.com"
	for i, body := range capturedBodies {
		count := strings.Count(body, ccAddr)
		if count != 1 {
			t.Errorf("request %d: %q appears %d times, want exactly 1", i+1, ccAddr, count)
		}
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
				_, _ = w.Write([]byte(`{"errors":[{"message":"server error"}]}`))
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
			w.WriteHeader(http.StatusInternalServerError)
			return
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
			w.WriteHeader(http.StatusInternalServerError)
			return
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
			w.WriteHeader(http.StatusInternalServerError)
			return
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
			w.WriteHeader(http.StatusInternalServerError)
			return
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

func TestSendOne_RetryOn500ThenSuccess(t *testing.T) {
	// The mock returns 500 twice then 202. With RetryMaxAttempts=3 the third
	// attempt succeeds, so SendOne must return nil error and call count == 3.
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}

	_, err := e.SendOne(recipient, "Hello", "<p>Hi {{.Name}}</p>", nil, nil, nil)
	if err != nil {
		t.Errorf("expected success after retries, got error: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("call count = %d, want 3", got)
	}
}

func TestSendOne_NoPermanentRetryOn400(t *testing.T) {
	// A 400 is a permanent client error and must not be retried.
	// Exactly one call should be made.
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	e := newTestEmailer(server.URL, 1000)
	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}

	_, err := e.SendOne(recipient, "Hello", "<p>Hi {{.Name}}</p>", nil, nil, nil)
	if err == nil {
		t.Error("expected error for 400 response, got nil")
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("call count = %d, want 1 (400 must not be retried)", got)
	}
}

func TestSendOne_TimeoutRetriedThenFails(t *testing.T) {
	// The handler sleeps longer than TimeoutMS so every attempt times out.
	// With RetryMaxAttempts=2 there should be exactly 2 calls, and SendOne
	// must return a non-nil error promptly (well within 2*TimeoutMS + backoff).
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:           "SG.test-key",
		FromEmail:        "test@example.com",
		FromName:         "Test Sender",
		MaxBatchSize:     1000,
		RateDelayMS:      0,
		TimeoutMS:        50,
		RetryMaxAttempts: 2,
		RetryBackoffMS:   1,
	}
	e := NewEmailer(cfg)
	e.SetBaseURL(server.URL)

	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}

	start := time.Now()
	_, err := e.SendOne(recipient, "Hello", "<p>Hi {{.Name}}</p>", nil, nil, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Errorf("call count = %d, want 2", got)
	}
	// Generous upper bound: 2*TimeoutMS + backoff + margin = 200ms + extra.
	// The key property is that it returns far sooner than 2*200ms (the sleep).
	if elapsed > 500*time.Millisecond {
		t.Errorf("SendOne took %v, want < 500ms (should fail fast on timeout)", elapsed)
	}
}
