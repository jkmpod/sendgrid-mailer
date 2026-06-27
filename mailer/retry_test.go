package mailer

import (
	"errors"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		wantDur time.Duration
		wantOK  bool
	}{
		{
			name:    "integer seconds header is parsed",
			headers: map[string][]string{"Retry-After": {"30"}},
			wantDur: 30 * time.Second,
			wantOK:  true,
		},
		{
			name:    "nil headers yields false",
			headers: nil,
			wantOK:  false,
		},
		{
			name:    "empty headers map yields false",
			headers: map[string][]string{},
			wantOK:  false,
		},
		{
			name:    "zero value yields false",
			headers: map[string][]string{"Retry-After": {"0"}},
			wantOK:  false,
		},
		{
			name:    "non-integer value yields false",
			headers: map[string][]string{"Retry-After": {"abc"}},
			wantOK:  false,
		},
		{
			name:    "negative value yields false",
			headers: map[string][]string{"Retry-After": {"-5"}},
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := retryAfter(tt.headers)
			if ok != tt.wantOK {
				t.Errorf("retryAfter() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantDur {
				t.Errorf("retryAfter() dur = %v, want %v", got, tt.wantDur)
			}
		})
	}
}

func TestNextDelay(t *testing.T) {
	const base = 500
	const cap30s = 30000

	tests := []struct {
		name       string
		statusCode int
		headers    map[string][]string
		attempt    int
		wantDelay  time.Duration
	}{
		{
			name:       "429 with Retry-After respects the header",
			statusCode: 429,
			headers:    map[string][]string{"Retry-After": {"10"}},
			attempt:    1,
			wantDelay:  10 * time.Second,
		},
		{
			name:       "429 with Retry-After larger than cap is capped",
			statusCode: 429,
			headers:    map[string][]string{"Retry-After": {"120"}},
			attempt:    1,
			wantDelay:  30 * time.Second,
		},
		{
			name:       "429 without Retry-After falls back to backoff",
			statusCode: 429,
			headers:    map[string][]string{},
			attempt:    1,
			wantDelay:  backoff(1, base),
		},
		{
			name:       "500 with Retry-After ignores header and uses backoff",
			statusCode: 500,
			headers:    map[string][]string{"Retry-After": {"10"}},
			attempt:    1,
			wantDelay:  backoff(1, base),
		},
		{
			name:       "status 0 with nil headers uses backoff",
			statusCode: 0,
			headers:    nil,
			attempt:    1,
			wantDelay:  backoff(1, base),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextDelay(tt.statusCode, tt.headers, tt.attempt, base, cap30s)
			if got != tt.wantDelay {
				t.Errorf("nextDelay() = %v, want %v", got, tt.wantDelay)
			}
		})
	}
}

func TestIsTransient(t *testing.T) {
	someErr := errors.New("network error")

	tests := []struct {
		name       string
		statusCode int
		err        error
		want       bool
	}{
		{name: "non-nil error is transient", statusCode: 0, err: someErr, want: true},
		{name: "429 rate-limited is transient", statusCode: 429, err: nil, want: true},
		{name: "500 server error is transient", statusCode: 500, err: nil, want: true},
		{name: "503 service unavailable is transient", statusCode: 503, err: nil, want: true},
		{name: "400 bad request is not transient", statusCode: 400, err: nil, want: false},
		{name: "401 unauthorised is not transient", statusCode: 401, err: nil, want: false},
		{name: "200 success is not transient", statusCode: 200, err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransient(tt.statusCode, tt.err)
			if got != tt.want {
				t.Errorf("isTransient(%d, %v) = %v, want %v", tt.statusCode, tt.err, got, tt.want)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	const base = 100

	tests := []struct {
		attempt int
		wantMS  int
	}{
		{attempt: 1, wantMS: 100},
		{attempt: 2, wantMS: 200},
		{attempt: 3, wantMS: 400},
		{attempt: 4, wantMS: 800},
		{attempt: 5, wantMS: 1600},
		{attempt: 6, wantMS: 3200},
		// Beyond this the cap kicks in.
		{attempt: 7, wantMS: 5000},
		{attempt: 8, wantMS: 5000},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff(tt.attempt, base)
			want := time.Duration(tt.wantMS) * time.Millisecond
			if got != want {
				t.Errorf("backoff(%d, %d) = %v, want %v", tt.attempt, base, got, want)
			}
		})
	}

	// Verify monotonic increase up to the cap.
	prev := backoff(1, base)
	for a := 2; a <= 6; a++ {
		cur := backoff(a, base)
		if cur <= prev {
			t.Errorf("backoff(%d, %d) = %v, want > %v (not monotonic)", a, base, cur, prev)
		}
		prev = cur
	}

	// Verify values beyond the cap are equal to the cap.
	cap5s := 5 * time.Second
	for a := 7; a <= 10; a++ {
		got := backoff(a, base)
		if got != cap5s {
			t.Errorf("backoff(%d, %d) = %v, want %v (cap)", a, base, got, cap5s)
		}
	}
}
