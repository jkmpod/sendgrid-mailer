package handlers

import (
	"net/http"

	"github.com/jkmpod/sendgrid-mailer/config"
)

// HandleConfig returns an http.HandlerFunc that responds with a JSON object
// containing the current test mode status. The UI uses this on page load to
// show or hide the "TEST MODE ACTIVE" badge.
func HandleConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"testMode":    cfg.TestMode,
			"lastSubject": GetLastSubject(),
			"sendLog":     GetSendLog(),
		})
	}
}
