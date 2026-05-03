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
			name:      "date range filter",
			query:     "?from_date=2026-03-01T00:00:00Z&to_date=2026-03-28T23:59:59Z",
			wantInURL: "BETWEEN TIMESTAMP",
		},
		{
			name:       "from_date without to_date is ignored",
			query:      "?from_date=2026-03-01T00:00:00Z",
			wantAbsent: "BETWEEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler pointing at the mock server instead of real SendGrid.
			handler := handleLogsWithBaseURL(sgServer.URL+"/v3/messages", "SG.test-key")

			req := httptest.NewRequest("GET", "/logs"+tt.query, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
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

func TestHandleLogs_SendGridError(t *testing.T) {
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
