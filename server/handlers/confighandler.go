package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/mailer"
)

// HandleConfig returns an http.HandlerFunc that responds with a JSON object
// containing the current effective configuration. The UI uses this on page
// load to populate all settings fields.
func HandleConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
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
		defer r.Body.Close()

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
			email := strings.TrimSpace(*req.FromEmail)
			if email == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "fromEmail cannot be empty",
				})
				return
			}
			if _, err := mail.ParseAddress(email); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("invalid fromEmail: %v", err),
				})
				return
			}
			SetRuntimeFromEmail(email)
			log.Printf("[config] from email set to %q", email)
		}
		if req.FromName != nil {
			name := strings.TrimSpace(*req.FromName)
			if name == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "fromName cannot be empty",
				})
				return
			}
			SetRuntimeFromName(name)
			log.Printf("[config] from name set to %q", name)
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
