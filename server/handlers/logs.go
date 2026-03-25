package handlers

import (
	"fmt"
	"io"
	"net/http"
)

// HandleLogs returns an http.HandlerFunc that calls the SendGrid Activity Feed
// API to fetch the last 50 message events and returns the raw JSON response
// to the client.
func HandleLogs(apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest("GET", "https://api.sendgrid.com/v3/messages?limit=50", nil)
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
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": fmt.Sprintf("failed to reach SendGrid API: %v", err),
			})
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
