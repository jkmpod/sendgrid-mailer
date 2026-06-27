package mailer

import (
	"strconv"
	"strings"
	"time"
)

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

// retryAfter parses the Retry-After response header (the integer-seconds form
// SendGrid returns on a 429) and reports whether a usable delay was found. The
// HTTP-date form is not parsed and yields ok=false (the caller then falls back
// to exponential backoff); a missing, non-integer, or non-positive value also
// yields ok=false.
func retryAfter(headers map[string][]string) (time.Duration, bool) {
	if headers == nil {
		return 0, false
	}
	vals := headers["Retry-After"]
	if len(vals) == 0 {
		return 0, false
	}
	secs, err := strconv.Atoi(strings.TrimSpace(vals[0]))
	if err != nil || secs <= 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

// nextDelay returns how long to wait before the next retry attempt. For a 429
// carrying a usable Retry-After header it respects that value, capped at capMS
// milliseconds; for every other transient case it uses exponential backoff.
func nextDelay(statusCode int, headers map[string][]string, attempt, baseMS, capMS int) time.Duration {
	if statusCode == 429 {
		if d, ok := retryAfter(headers); ok {
			capDur := time.Duration(capMS) * time.Millisecond
			if d > capDur {
				d = capDur
			}
			return d
		}
	}
	return backoff(attempt, baseMS)
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
