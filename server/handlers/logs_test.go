package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleLogs(t *testing.T) {
	// Mock SendGrid API server that echoes back the request URL
	// so we can verify the query string construction.
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return the raw URL so tests can inspect it.
		if err := json.NewEncoder(w).Encode(map[string]string{
			"requestURL": r.URL.String(),
		}); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
	defer sgServer.Close()

	tests := []struct {
		name       string
		query      string // query string appended to /logs
		wantInURL  string // substring expected in the forwarded URL
		wantAbsent string // substring that must NOT appear (empty = skip check)
		wantStatus int    // expected HTTP status code (defaults to 200)
	}{
		{
			name:      "default limit",
			query:     "",
			wantInURL: "limit=50",
		},
		{
			name:      "custom limit",
			query:     "?limit=200",
			wantInURL: "limit=200",
		},
		{
			name:      "limit capped at 1000",
			query:     "?limit=5000",
			wantInURL: "limit=1000",
		},
		{
			name:      "invalid limit falls back to 50",
			query:     "?limit=abc",
			wantInURL: "limit=50",
		},
		{
			name:      "negative limit falls back to 50",
			query:     "?limit=-10",
			wantInURL: "limit=50",
		},
		{
			name:      "subject filter",
			query:     "?subject=Hello",
			wantInURL: `subject="Hello"`,
		},
		{
			name:       "subject with double-quote returns 400",
			query:      `?subject=Hello%22World`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:      "status filter not_delivered",
			query:     "?status=not_delivered",
			wantInURL: `status="not_delivered"`,
		},
		{
			name:       "invalid status is ignored",
			query:      "?status=hacked",
			wantAbsent: "status=",
		},
		{
			name:       "unsupported granular status bounced is ignored",
			query:      "?status=bounced",
			wantAbsent: "status=",
		},
		{
			name:      "combined subject and status",
			query:     "?subject=Hi&status=delivered",
			wantInURL: " AND ",
		},
		{
			name:      "recipient filter",
			query:     "?to_email=user@example.com",
			wantInURL: `to_email="user@example.com"`,
		},
		{
			name:       "invalid to_email returns 400",
			query:      "?to_email=not-an-email",
			wantStatus: http.StatusBadRequest,
		},
		{
			// ParseAddress accepts display-name forms such as "x" <a@b.com>,
			// which contain a literal double-quote. The handler must normalise
			// to the bare addr.Address so only a@b.com reaches the DSL clause.
			name:       "display-name to_email normalised to bare address",
			query:      `?to_email=%22x%22+%3Ca%40b.com%3E`, // "x" <a@b.com>
			wantInURL:  `to_email="a@b.com"`,
			wantAbsent: `"x"`,
		},
		{
			name:      "date range filter",
			query:     "?from_date=2026-03-01T00:00:00Z&to_date=2026-03-28T23:59:59Z",
			wantInURL: "BETWEEN TIMESTAMP",
		},
		{
			name:       "from_date without to_date returns 400",
			query:      "?from_date=2026-03-01T00:00:00Z",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "to_date without from_date returns 400",
			query:      "?to_date=2026-03-28T23:59:59Z",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid from_date returns 400",
			query:      "?from_date=invalid-date",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid to_date returns 400",
			query:      "?from_date=2026-03-01T00:00:00Z&to_date=not-a-date",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Default wantStatus to 200 if not specified.
			wantStatus := tt.wantStatus
			if wantStatus == 0 {
				wantStatus = http.StatusOK
			}

			// Create handler pointing at the mock server instead of real SendGrid.
			handler := handleLogsWithBaseURL(sgServer.URL+"/v3/messages", "SG.test-key")

			req := httptest.NewRequest("GET", "/logs"+tt.query, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rr.Code, wantStatus, rr.Body.String())
			}

			if rr.Code != http.StatusOK {
				// For non-200, just ensure it's JSON with an error field.
				var resp map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if resp["error"] == "" {
					t.Errorf("expected non-empty error field in response: %v", resp)
				}
				return
			}

			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			// URL-decode so we can match against readable query strings.
			gotURL, _ := url.QueryUnescape(resp["requestURL"])
			if tt.wantInURL != "" && !strings.Contains(gotURL, tt.wantInURL) {
				t.Errorf("URL = %q, want substring %q", gotURL, tt.wantInURL)
			}
			if tt.wantAbsent != "" && strings.Contains(gotURL, tt.wantAbsent) {
				t.Errorf("URL = %q, should NOT contain %q", gotURL, tt.wantAbsent)
			}
		})
	}
}

func TestHandleLogs_SendGridAPIError(t *testing.T) {
	// Mock SendGrid API server that returns 401 Unauthorised.
	sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain") // Not JSON
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Permission denied"))
	}))
	defer sgServer.Close()

	handler := handleLogsWithBaseURL(sgServer.URL, "SG.test-key")

	req := httptest.NewRequest("GET", "/logs", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// Handler should return a JSON error wrapping the upstream status.
	if !strings.Contains(resp["error"], "SendGrid API returned status 401") {
		t.Errorf("error = %q, want substring %q", resp["error"], "SendGrid API returned status 401")
	}
	// Truncated upstream body must appear as the detail field.
	if resp["detail"] != "Permission denied" {
		t.Errorf("detail = %q, want %q", resp["detail"], "Permission denied")
	}
}

func TestHandleLogs_SendGridNetworkError(t *testing.T) {
	// Point at a server that immediately closes the connection.
	handler := handleLogsWithBaseURL("http://127.0.0.1:1", "SG.test-key")

	req := httptest.NewRequest("GET", "/logs", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !strings.Contains(resp["error"], "failed to reach SendGrid API") {
		t.Errorf("error = %q, want substring %q", resp["error"], "failed to reach SendGrid API")
	}
}
