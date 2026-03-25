package handlers

import "sync"

// subjectMu guards lastSubject. A mutex is needed because HTTP handlers run
// concurrently in separate goroutines. Without a mutex, one goroutine could
// read lastSubject while another is mid-write, causing a data race — even
// for a simple string variable. Go's race detector will flag this.
var (
	subjectMu   sync.Mutex
	lastSubject string
)

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
