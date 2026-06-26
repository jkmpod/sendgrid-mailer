package mailer

import (
	"errors"
	"testing"
	"time"
)

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
