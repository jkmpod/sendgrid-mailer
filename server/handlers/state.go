package handlers

import (
	"sync"
	"time"
)

// subjectMu guards lastSubject and sendLog. A mutex is needed because HTTP
// handlers run concurrently in separate goroutines. Without a mutex, one
// goroutine could read while another is mid-write, causing a data race —
// even for a simple string variable. Go's race detector will flag this.
var (
	subjectMu   sync.Mutex
	lastSubject string
	sendLog     []SendLogEntry
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
