package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/mailer"
)

// HandleConfig returns an http.HandlerFunc that responds with a JSON object
// containing the current effective configuration. The UI uses this on page
// load to populate all settings fields.
func HandleConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"testMode":    EffectiveTestMode(cfg),
			"testEmails":  EffectiveTestEmails(cfg),
			"fromEmail":   EffectiveFromEmail(cfg),
			"fromName":    EffectiveFromName(cfg),
			"lastSubject": GetLastSubject(),
			"sendLog":     GetSendLog(),
		})
	}
}

// configUpdateRequest holds optional fields for runtime config changes.
// Pointer types allow distinguishing "not sent" from "set to zero value".
type configUpdateRequest struct {
	TestMode   *bool    `json:"testMode"`
	TestEmails []string `json:"testEmails"`
	FromEmail  *string  `json:"fromEmail"`
	FromName   *string  `json:"fromName"`
}

// HandleConfigUpdate returns an http.HandlerFunc that accepts a JSON POST
// to update runtime configuration. Changes are in-memory only and reset
// on app restart.
func HandleConfigUpdate(e *mailer.Emailer, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req configUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

		if req.TestMode != nil {
			SetRuntimeTestMode(*req.TestMode)
			log.Printf("[config] test mode set to %v", *req.TestMode)
		}
		if req.TestEmails != nil {
			SetRuntimeTestEmails(req.TestEmails)
			log.Printf("[config] test emails set to %v", req.TestEmails)
		}
		if req.FromEmail != nil {
			SetRuntimeFromEmail(*req.FromEmail)
			log.Printf("[config] from email set to %q", *req.FromEmail)
		}
		if req.FromName != nil {
			SetRuntimeFromName(*req.FromName)
			log.Printf("[config] from name set to %q", *req.FromName)
		}

		// Sync from address to Emailer if either field was updated.
		if req.FromEmail != nil || req.FromName != nil {
			e.SetFrom(EffectiveFromEmail(cfg), EffectiveFromName(cfg))
		}

		// Return the full current state.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"testMode":    EffectiveTestMode(cfg),
			"testEmails":  EffectiveTestEmails(cfg),
			"fromEmail":   EffectiveFromEmail(cfg),
			"fromName":    EffectiveFromName(cfg),
			"lastSubject": GetLastSubject(),
			"sendLog":     GetSendLog(),
		})
	}
}
