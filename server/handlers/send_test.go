package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/mailer"
)

// writeTempCSV creates a temporary CSV file and returns its path.
func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp CSV: %v", err)
	}
	return path
}

func TestHandleSend(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		csvContent string // if empty, don't create a temp file
		wantStatus int
		wantErr    string
	}{
		{
			name:       "missing filePath",
			body:       `{"subject":"Hi","template":"<p>Hello</p>"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "filePath is required",
		},
		{
			name:       "missing subject",
			body:       `{"filePath":"/tmp/test.csv","template":"<p>Hello</p>"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "subject is required",
		},
		{
			name:       "missing template",
			body:       `{"filePath":"/tmp/test.csv","subject":"Hi"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "template is required",
		},
		{
			name:       "invalid JSON body",
			body:       `not json`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "invalid JSON body",
		},
		{
			name:       "CSV file not found",
			body:       `{"subject":"Hi","template":"<p>Hello</p>","filePath":"/nonexistent/file.csv"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "failed to load CSV",
		},
	}

	// Create a mock SendGrid server (won't be reached for validation errors).
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer sgServer.Close()

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
		TestMode:     false,
	}
	e := mailer.NewEmailer(cfg)
	e.SetBaseURL(sgServer.URL)
	handler := HandleSend(e, cfg)
	ResetRuntimeConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/send", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantErr != "" {
				var resp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				errMsg, ok := resp["error"].(string)
				if !ok {
					t.Fatalf("expected error field, got: %v", resp)
				}
				if !strings.Contains(errMsg, tt.wantErr) {
					t.Errorf("error = %q, want substring %q", errMsg, tt.wantErr)
				}
			}
		})
	}
}

func TestHandleSend_ValidRequest(t *testing.T) {
	// Mock SendGrid returns 202 Accepted for all requests.
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer sgServer.Close()

	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\nbob@example.com,Bob\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)
	e.SetBaseURL(sgServer.URL)

	body := `{"subject":"Hello","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`

	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	responseBody := rr.Body.String()

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") && !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want text/event-stream or application/json", ct)
	}

	// With the mock server wired in, all 2 recipients succeed.
	// Expect a "done" event with totalSent=2 and totalFailed=0.
	if !strings.Contains(responseBody, "done") {
		t.Errorf("expected 'done' event in response; got: %s", responseBody)
	}
	if !strings.Contains(responseBody, `"totalSent":2`) {
		t.Errorf("expected totalSent:2 in done event; got: %s", responseBody)
	}
	if !strings.Contains(responseBody, `"totalFailed":0`) {
		t.Errorf("expected totalFailed:0 in done event; got: %s", responseBody)
	}
}

func TestHandleSend_PartialFailure(t *testing.T) {
	// Mock server returns 202 on the first POST and 500 on the second.
	var callCount int32
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer sgServer.Close()

	// 3 recipients with batch size 2 → 2 batches: [alice, bob] and [charlie].
	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\nbob@example.com,Bob\ncharlie@example.com,Charlie\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 2,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)
	e.SetBaseURL(sgServer.URL)

	body := `{"subject":"Hello","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`

	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	responseBody := rr.Body.String()

	// Batch 1 (alice+bob) succeeds; batch 2 (charlie) fails.
	if !strings.Contains(responseBody, `"batch":1`) {
		t.Errorf("expected batch 1 event; got: %s", responseBody)
	}
	if !strings.Contains(responseBody, `"batch":2`) {
		t.Errorf("expected batch 2 event; got: %s", responseBody)
	}
	if !strings.Contains(responseBody, "done") {
		t.Errorf("expected 'done' event; got: %s", responseBody)
	}
	// Batch 1 ok → totalSent=2; batch 2 failed → totalFailed=1.
	if !strings.Contains(responseBody, `"totalSent":2`) {
		t.Errorf("expected totalSent:2 in done event; got: %s", responseBody)
	}
	if !strings.Contains(responseBody, `"totalFailed":1`) {
		t.Errorf("expected totalFailed:1 in done event; got: %s", responseBody)
	}
}

func TestHandleSend_LastSubjectNotUpdatedOnFailure(t *testing.T) {
	// Clear any previous state and set a known value.
	SetLastSubject("previous subject")

	// Create a CSV that references a non-existent path to trigger a failure
	// before any send happens.
	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)

	body := `{"subject":"New Subject","template":"<p>Hi</p>","filePath":"/nonexistent/file.csv"}`
	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	// The request fails (CSV not found), so lastSubject should remain unchanged.
	got := GetLastSubject()
	if got != "previous subject" {
		t.Errorf("GetLastSubject() = %q, want %q (should not be updated on failure)", got, "previous subject")
	}
}

func TestValidateCategories(t *testing.T) {
	tests := []struct {
		name       string
		input      []string
		wantOut    []string
		wantErrSub string // non-empty means expect an error containing this substring
	}{
		{
			name:    "nil input returns empty slice",
			input:   nil,
			wantOut: []string{},
		},
		{
			name:    "empty input returns empty slice",
			input:   []string{},
			wantOut: []string{},
		},
		{
			name:    "single valid category",
			input:   []string{"newsletter"},
			wantOut: []string{"newsletter"},
		},
		{
			name:    "whitespace is trimmed",
			input:   []string{"  newsletter  ", " march-2026 "},
			wantOut: []string{"newsletter", "march-2026"},
		},
		{
			name:    "empty entries are dropped",
			input:   []string{"a", "", "  ", "b"},
			wantOut: []string{"a", "b"},
		},
		{
			name:    "duplicates are deduped preserving first occurrence",
			input:   []string{"a", "a", "b"},
			wantOut: []string{"a", "b"},
		},
		{
			name:       "category exceeding 255 chars returns error",
			input:      []string{strings.Repeat("x", 256)},
			wantErrSub: "exceeds 255 characters",
		},
		{
			name:       "more than 10 categories returns error",
			input:      []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			wantErrSub: "maximum is 10",
		},
		{
			name:    "exactly 10 categories is valid",
			input:   []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			wantOut: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := validateCategories(tt.input)
			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out) != len(tt.wantOut) {
				t.Fatalf("len(out) = %d, want %d; got %v", len(out), len(tt.wantOut), out)
			}
			for i, want := range tt.wantOut {
				if out[i] != want {
					t.Errorf("out[%d] = %q, want %q", i, out[i], want)
				}
			}
		})
	}
}

func TestHandleSend_Categories(t *testing.T) {
	csvContent := "email,name\nalice@example.com,Alice\n"

	tests := []struct {
		name       string
		categories interface{} // included in the JSON body
		wantStatus int
		wantErrSub string
	}{
		{
			name:       "valid single category returns 200",
			categories: []string{"newsletter"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "11 categories returns 400",
			categories: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			wantStatus: http.StatusBadRequest,
			wantErrSub: "maximum is 10",
		},
		{
			name:       "256-char category returns 400",
			categories: []string{strings.Repeat("x", 256)},
			wantStatus: http.StatusBadRequest,
			wantErrSub: "exceeds 255 characters",
		},
	}

	// Mock SendGrid returns 202 for valid requests.
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer sgServer.Close()

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
		TestMode:     false,
	}
	e := mailer.NewEmailer(cfg)
	e.SetBaseURL(sgServer.URL)
	handler := HandleSend(e, cfg)
	ResetRuntimeConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csvPath := writeTempCSV(t, csvContent)

			type reqBody struct {
				Subject    string      `json:"subject"`
				Template   string      `json:"template"`
				FilePath   string      `json:"filePath"`
				Categories interface{} `json:"categories,omitempty"`
			}
			rb := reqBody{
				Subject:    "Hello",
				Template:   "<p>Hi {{.Name}}</p>",
				FilePath:   csvPath,
				Categories: tt.categories,
			}
			bodyBytes, _ := json.Marshal(rb)

			req := httptest.NewRequest("POST", "/send", strings.NewReader(string(bodyBytes)))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if tt.wantErrSub != "" {
				var resp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				errMsg, _ := resp["error"].(string)
				if !strings.Contains(errMsg, tt.wantErrSub) {
					t.Errorf("error = %q, want substring %q", errMsg, tt.wantErrSub)
				}
			}
		})
	}
}

func TestHandleSend_LastSubjectUpdatedOnSuccess(t *testing.T) {
	SetLastSubject("")

	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer sgServer.Close()

	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)
	e.SetBaseURL(sgServer.URL)

	body := `{"subject":"Success Subject","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`
	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	_ = rr // response used for side-effects

	// Mock returns 202, so totalSent=1 → lastSubject should be updated.
	got := GetLastSubject()
	if got != "Success Subject" {
		t.Errorf("GetLastSubject() = %q, want %q", got, "Success Subject")
	}
}

func TestHandleSend_LastSubjectNotUpdatedOnAllFailure(t *testing.T) {
	SetLastSubject("previous subject")

	// Mock SendGrid always returns 500.
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer sgServer.Close()

	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)
	e.SetBaseURL(sgServer.URL)

	body := `{"subject":"Failing Subject","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`
	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	_ = rr

	// All batches fail (500), so totalSent=0 → lastSubject must not change.
	got := GetLastSubject()
	if got != "previous subject" {
		t.Errorf("GetLastSubject() = %q, want %q (should not update when totalSent=0)", got, "previous subject")
	}
}
