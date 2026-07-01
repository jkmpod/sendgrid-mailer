package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// validStatuses is the set of status values accepted by the SendGrid
// Email Activity Feed API query language. SendGrid collapses all failure
// modes (bounces, blocks, deferrals, spam reports) into "not_delivered".
// Only these three values are valid — event-level granularity (bounced,
// blocked, etc.) is not queryable via the status filter.
var validStatuses = map[string]bool{
	"delivered":     true,
	"not_delivered": true,
	"processing":    true,
}

const (
	defaultLogLimit      = 50
	maxLogLimit          = 1000
	maxUpstreamErrDetail = 512
)

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
// Multiple filters are combined with AND. messagesURL is the base URL for the
// SendGrid Activity Feed API, read from cfg.MessagesURL.
func HandleLogs(apiKey, messagesURL string) http.HandlerFunc {
	return handleLogsWithBaseURL(messagesURL, apiKey)
}

// handleLogsWithBaseURL is the core implementation. The baseURL parameter
// allows tests to point at a mock server instead of the real SendGrid API.
func handleLogsWithBaseURL(baseURL, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		q := r.URL.Query()

		// Parse limit (default 50, max 1000).
		limit := defaultLogLimit
		if l := q.Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
				if limit > maxLogLimit {
					limit = maxLogLimit
				}
			}
		}

		// Validate filter parameters before building the request URL.
		// subject: SendGrid DSL has no documented quote-escaping, so reject
		// values that contain a double-quote to prevent clause breakout.
		subject := q.Get("subject")
		if strings.Contains(subject, `"`) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": `subject may not contain a double-quote`,
			})
			return
		}

		// to_email: parse with net/mail and interpolate the normalised bare
		// address (addr.Address) rather than the raw input. ParseAddress
		// accepts display-name forms such as `"Name" <user@example.com>`,
		// which contain a literal double-quote that would break out of the
		// DSL clause. Using addr.Address strips the display name and yields a
		// quote-free local@domain string safe to interpolate.
		toEmail := q.Get("to_email")
		if toEmail != "" {
			addr, err := mail.ParseAddress(toEmail)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("invalid to_email: %v", err),
				})
				return
			}
			toEmail = addr.Address // normalised bare email, no display-name quotes
		}

		sgURL := fmt.Sprintf("%s?limit=%d", baseURL, limit)

		// Build query clauses from filter parameters.
		var clauses []string

		if subject != "" {
			clauses = append(clauses, fmt.Sprintf(`subject="%s"`, subject))
		}
		if status := q.Get("status"); status != "" && validStatuses[status] {
			clauses = append(clauses, fmt.Sprintf(`status="%s"`, status))
		}
		if toEmail != "" {
			clauses = append(clauses, fmt.Sprintf(`to_email="%s"`, toEmail))
		}

		fromDate := q.Get("from_date")
		if fromDate != "" {
			if _, err := time.Parse(time.RFC3339, fromDate); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("invalid from_date: %v", err),
				})
				return
			}
		}

		toDate := q.Get("to_date")
		if toDate != "" {
			if _, err := time.Parse(time.RFC3339, toDate); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("invalid to_date: %v", err),
				})
				return
			}
		}

		// Both dates must be supplied together.
		if (fromDate != "") != (toDate != "") {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "from_date and to_date must be provided together",
			})
			return
		}

		if fromDate != "" && toDate != "" {
			clauses = append(clauses, fmt.Sprintf(
				`last_event_time BETWEEN TIMESTAMP "%s" AND TIMESTAMP "%s"`,
				fromDate, toDate))
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

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[logs] SendGrid error response (status %d): %s", resp.StatusCode, string(body))
			detail := string(body)
			if len(body) > maxUpstreamErrDetail {
				detail = string(body[:maxUpstreamErrDetail])
			}
			writeJSON(w, resp.StatusCode, map[string]string{
				"error":  fmt.Sprintf("SendGrid API returned status %d", resp.StatusCode),
				"detail": detail,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("[logs] failed to stream response body to client: %v", err)
		}
	}
}
