package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// validStatuses is the set of status values accepted by the SendGrid
// Activity Feed API. We validate against this to prevent query injection.
var validStatuses = map[string]bool{
	"delivered":     true,
	"not_delivered": true,
	"bounced":       true,
	"blocked":       true,
	"deferred":      true,
	"processing":    true,
	"spam_reported": true,
}

const sendgridMessagesURL = "https://api.sendgrid.com/v3/messages"

// HandleLogs returns an http.HandlerFunc that calls the SendGrid Activity Feed
// API and returns the raw JSON response to the client. It accepts optional
// query parameters to filter results:
//
//	?limit=N        — number of results (default 50, max 1000)
//	?subject=...    — filter by subject line
//	?status=...     — filter by delivery status (delivered, bounced, blocked, etc.)
//	?to_email=...   — filter by recipient email
//	?from_date=...  — start of date range (ISO 8601 timestamp)
//	?to_date=...    — end of date range (ISO 8601 timestamp)
//
// Multiple filters are combined with AND.
func HandleLogs(apiKey string) http.HandlerFunc {
	return handleLogsWithBaseURL(sendgridMessagesURL, apiKey)
}

// handleLogsWithBaseURL is the core implementation. The baseURL parameter
// allows tests to point at a mock server instead of the real SendGrid API.
func handleLogsWithBaseURL(baseURL, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		// Parse limit (default 50, max 1000).
		limit := 50
		if l := q.Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
				if limit > 1000 {
					limit = 1000
				}
			}
		}

		sgURL := fmt.Sprintf("%s?limit=%d", baseURL, limit)

		// Build query clauses from filter parameters.
		var clauses []string

		if subject := q.Get("subject"); subject != "" {
			clauses = append(clauses, fmt.Sprintf(`subject="%s"`, subject))
		}
		if status := q.Get("status"); status != "" && validStatuses[status] {
			clauses = append(clauses, fmt.Sprintf(`status="%s"`, status))
		}
		if toEmail := q.Get("to_email"); toEmail != "" {
			clauses = append(clauses, fmt.Sprintf(`to_email="%s"`, toEmail))
		}
		if fromDate := q.Get("from_date"); fromDate != "" {
			if toDate := q.Get("to_date"); toDate != "" {
				clauses = append(clauses, fmt.Sprintf(
					`last_event_time BETWEEN TIMESTAMP "%s" AND TIMESTAMP "%s"`,
					fromDate, toDate))
			}
		}

		if len(clauses) > 0 {
			query := strings.Join(clauses, " AND ")
			sgURL += "&query=" + url.QueryEscape(query)
		}

		log.Printf("[logs] fetching SendGrid activity: url=%s", sgURL)

		req, err := http.NewRequest("GET", sgURL, nil)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to create request to SendGrid",
			})
			return
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[logs] SendGrid API error: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": fmt.Sprintf("failed to reach SendGrid API: %v", err),
			})
			return
		}
		defer resp.Body.Close()

		log.Printf("[logs] SendGrid response: status=%d", resp.StatusCode)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
