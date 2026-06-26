package mailer

import "time"

// isTransient reports whether a send attempt should be retried.
// It returns true when err is non-nil (network error or context timeout),
// statusCode is 429 (rate-limited), or statusCode is 500 or above (server
// error). Permanent client errors — any other 4xx — return false and are
// not retried.
func isTransient(statusCode int, err error) bool {
	if err != nil {
		return true
	}
	return statusCode == 429 || statusCode >= 500
}

// backoff returns the delay to wait before attempt number attempt (1-indexed).
// The delay is base * 2^(attempt-1), capped at five seconds. It is
// deterministic — no random jitter — so tests can rely on the exact values.
func backoff(attempt, baseMS int) time.Duration {
	ms := baseMS
	for i := 1; i < attempt; i++ {
		ms *= 2
	}
	const capMS = 5000
	if ms > capMS {
		ms = capMS
	}
	return time.Duration(ms) * time.Millisecond
}
