package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

// HandleLogs returns an http.HandlerFunc that calls the SendGrid Activity Feed
// API to fetch the last 50 message events and returns the raw JSON response
// to the client. If a ?subject= query param is provided, it is passed to the
// SendGrid API as a query filter.
func HandleLogs(apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sgURL := "https://api.sendgrid.com/v3/messages?limit=50"

		subject := r.URL.Query().Get("subject")
		if subject != "" {
			// SendGrid Activity Feed uses a query language:
			// subject="value" filters by subject line.
			sgURL += "&query=" + url.QueryEscape(`subject="`+subject+`"`)
		}

		log.Printf("[logs] fetching SendGrid activity: subject=%q url=%s", subject, sgURL)

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
