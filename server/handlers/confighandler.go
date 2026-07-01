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
// on app restart. All fields are validated before any mutation is applied,
// so a rejected request performs zero state changes.
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

		// Validation phase: check every field before applying any mutation.
		var trimmedEmail, trimmedName string
		if req.FromEmail != nil {
			trimmedEmail = strings.TrimSpace(*req.FromEmail)
			if trimmedEmail == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "fromEmail cannot be empty",
				})
				return
			}
			if _, err := mail.ParseAddress(trimmedEmail); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("invalid fromEmail: %v", err),
				})
				return
			}
		}
		if req.FromName != nil {
			// Empty name is accepted (not rejected). Note: the runtime override
			// treats "" as "unset", so an empty value falls back to the
			// configured FROM_NAME in EffectiveFromName rather than clearing it.
			trimmedName = strings.TrimSpace(*req.FromName)
		}

		// Apply phase: only reached when all validation has passed.
		if req.TestMode != nil {
			SetRuntimeTestMode(*req.TestMode)
			log.Printf("[config] test mode set to %v", *req.TestMode)
		}
		if req.TestEmails != nil {
			SetRuntimeTestEmails(req.TestEmails)
			log.Printf("[config] test emails set to %v", req.TestEmails)
		}
		if req.FromEmail != nil {
			SetRuntimeFromEmail(trimmedEmail)
			log.Printf("[config] from email set to %q", trimmedEmail)
		}
		if req.FromName != nil {
			SetRuntimeFromName(trimmedName)
			log.Printf("[config] from name set to %q", trimmedName)
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
