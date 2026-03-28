package handlers

import (
	"sync"
	"time"

	"github.com/jkmpod/sendgrid-mailer/config"
)

// subjectMu guards lastSubject and sendLog. A mutex is needed because HTTP
// handlers run concurrently in separate goroutines. Without a mutex, one
// goroutine could read while another is mid-write, causing a data race —
// even for a simple string variable. Go's race detector will flag this.
var (
	subjectMu   sync.Mutex
	lastSubject string
	sendLog     []SendLogEntry

	// Runtime config overrides — nil/empty means "use Config default".
	// Set via POST /config, reset on app restart.
	runtimeTestMode   *bool
	runtimeTestEmails []string
	runtimeFromEmail  string
	runtimeFromName   string
)

// SendLogEntry records the outcome of a single send operation.
// Stored in memory only — lost on restart.
type SendLogEntry struct {
	Time        time.Time `json:"time"`
	Subject     string    `json:"subject"`
	TotalSent   int       `json:"totalSent"`
	TotalFailed int       `json:"totalFailed"`
	TestMode    bool      `json:"testMode"`
}

// SetLastSubject stores the subject line of the most recent successful send.
func SetLastSubject(s string) {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	lastSubject = s
}

// GetLastSubject returns the subject line of the most recent successful send.
// It returns an empty string if no send has happened yet.
func GetLastSubject() string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	return lastSubject
}

// AppendSendLog adds an entry to the in-memory send log.
// The log is capped at 50 entries — oldest entries are dropped first.
func AppendSendLog(entry SendLogEntry) {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	sendLog = append(sendLog, entry)
	if len(sendLog) > 50 {
		sendLog = sendLog[len(sendLog)-50:]
	}
}

// GetSendLog returns a copy of the in-memory send log.
// Returns an empty slice (not nil) if no sends have occurred.
func GetSendLog() []SendLogEntry {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	copied := make([]SendLogEntry, len(sendLog))
	copy(copied, sendLog)
	return copied
}

// ResetSendLog clears the in-memory send log. Used by tests.
func ResetSendLog() {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	sendLog = nil
}

// --- Runtime config overrides ---

// SetRuntimeTestMode overrides the test mode setting from config.
func SetRuntimeTestMode(v bool) {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	runtimeTestMode = &v
}

// GetRuntimeTestMode returns the override value, or nil if not set.
func GetRuntimeTestMode() *bool {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	return runtimeTestMode
}

// SetRuntimeTestEmails overrides the test email list from config.
func SetRuntimeTestEmails(v []string) {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	runtimeTestEmails = v
}

// GetRuntimeTestEmails returns the override value, or nil if not set.
func GetRuntimeTestEmails() []string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	return runtimeTestEmails
}

// SetRuntimeFromEmail overrides the sender email from config.
func SetRuntimeFromEmail(v string) {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	runtimeFromEmail = v
}

// GetRuntimeFromEmail returns the override value, or "" if not set.
func GetRuntimeFromEmail() string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	return runtimeFromEmail
}

// SetRuntimeFromName overrides the sender name from config.
func SetRuntimeFromName(v string) {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	runtimeFromName = v
}

// GetRuntimeFromName returns the override value, or "" if not set.
func GetRuntimeFromName() string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	return runtimeFromName
}

// ResetRuntimeConfig clears all runtime overrides. Used by tests.
func ResetRuntimeConfig() {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	runtimeTestMode = nil
	runtimeTestEmails = nil
	runtimeFromEmail = ""
	runtimeFromName = ""
}

// --- Effective value resolution ---
// These return the runtime override if set, otherwise the Config default.

// EffectiveTestMode returns the runtime override if set, else cfg.TestMode.
func EffectiveTestMode(cfg *config.Config) bool {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	if runtimeTestMode != nil {
		return *runtimeTestMode
	}
	return cfg.TestMode
}

// EffectiveTestEmails returns the runtime override if set, else cfg.TestEmails.
func EffectiveTestEmails(cfg *config.Config) []string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	if runtimeTestEmails != nil {
		return runtimeTestEmails
	}
	return cfg.TestEmails
}

// EffectiveFromEmail returns the runtime override if non-empty, else cfg.FromEmail.
func EffectiveFromEmail(cfg *config.Config) string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	if runtimeFromEmail != "" {
		return runtimeFromEmail
	}
	return cfg.FromEmail
}

// EffectiveFromName returns the runtime override if non-empty, else cfg.FromName.
func EffectiveFromName(cfg *config.Config) string {
	subjectMu.Lock()
	defer subjectMu.Unlock()
	if runtimeFromName != "" {
		return runtimeFromName
	}
	return cfg.FromName
}
