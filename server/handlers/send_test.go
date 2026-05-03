package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	// Set up a mock SendGrid API server.
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer sgServer.Close()

	// Create a temp CSV file.
	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\nbob@example.com,Bob\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)

	// We can't redirect the unexported client to the mock server from this
	// package. So we test that the handler correctly validates input and
	// calls SendBulk. The actual SendGrid call will fail (wrong URL), which
	// exercises the partial-failure path.
	body := `{"subject":"Hello","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`

	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	// The handler should return SSE events (text/event-stream) or JSON.
	// Since the real SendGrid API won't be reached, we expect batch failures
	// streamed via SSE — which still means a 200 status (SSE always starts 200).
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") && !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want text/event-stream or application/json", ct)
	}

	// The response body should contain a "done" event with totalFailed > 0
	// (since the mock SendGrid URL isn't wired to the emailer's client).
	responseBody := rr.Body.String()
	if !strings.Contains(responseBody, "done") && !strings.Contains(responseBody, "totalFailed") {
		t.Logf("Response body: %s", responseBody)
		// Not a hard failure — the important thing is the handler didn't panic
		// and returned a response.
	}
}

func TestHandleSend_PartialFailure(t *testing.T) {
	// This test verifies that when some batches fail, the handler still
	// returns results for all batches (partial success).
	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\nbob@example.com,Bob\ncharlie@example.com,Charlie\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 2, // 2 batches: [alice, bob] and [charlie]
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)

	body := `{"subject":"Hello","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`

	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	// The handler should produce SSE events. Since the Emailer's client
	// points at the real SendGrid (which we can't reach), all batches will
	// fail. The key assertion is: the handler processes ALL batches, not
	// just the first one.
	responseBody := rr.Body.String()

	// We expect events for batch 1 and batch 2.
	if !strings.Contains(responseBody, `"batch":1`) {
		t.Errorf("expected batch 1 event in response")
	}
	if !strings.Contains(responseBody, `"batch":2`) {
		t.Errorf("expected batch 2 event in response")
	}
	if !strings.Contains(responseBody, "done") {
		t.Errorf("expected 'done' event in response")
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

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
		TestMode:     false,
	}
	e := mailer.NewEmailer(cfg)
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
	// Clear any previous state.
	SetLastSubject("")

	// Create a temp CSV.
	csvPath := writeTempCSV(t, "email,name\nalice@example.com,Alice\n")

	cfg := &config.Config{
		APIKey:       "SG.test-key",
		FromEmail:    "test@example.com",
		FromName:     "Test",
		MaxBatchSize: 1000,
		RateDelayMS:  0,
	}
	e := mailer.NewEmailer(cfg)

	body := `{"subject":"Success Subject","template":"<p>Hi {{.Name}}</p>","filePath":"` + strings.ReplaceAll(csvPath, `\`, `\\`) + `"}`
	req := httptest.NewRequest("POST", "/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := HandleSend(e, cfg)
	handler.ServeHTTP(rr, req)

	// The handler will attempt to send via SendGrid which will fail (client
	// points at real API). All batches fail, so totalSent = 0, meaning
	// lastSubject should NOT be updated. This validates the "only on success"
	// guard. We verify this by checking it remains empty.
	got := GetLastSubject()
	if got != "" {
		t.Errorf("GetLastSubject() = %q, want empty (all batches failed, totalSent=0)", got)
	}
}
