package handlers

import (
	"sync"
	"testing"
)

func TestSetGetLastSubject(t *testing.T) {
	// Reset state before tests.
	SetLastSubject("")

	tests := []struct {
		name     string
		setVal   string
		wantVal  string
		skipSet  bool // if true, don't call SetLastSubject
	}{
		{
			name:    "default value is empty string",
			skipSet: true,
			wantVal: "",
		},
		{
			name:    "set and get returns same value",
			setVal:  "Welcome to our newsletter",
			wantVal: "Welcome to our newsletter",
		},
		{
			name:    "overwrite with new value",
			setVal:  "Updated subject",
			wantVal: "Updated subject",
		},
		{
			name:    "set to empty string",
			setVal:  "",
			wantVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.skipSet {
				SetLastSubject(tt.setVal)
			}
			got := GetLastSubject()
			if got != tt.wantVal {
				t.Errorf("GetLastSubject() = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

func TestSetGetLastSubject_Concurrent(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	// Run many concurrent reads and writes to trigger the race detector.
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetLastSubject("concurrent subject")
		}()
		go func() {
			defer wg.Done()
			_ = GetLastSubject()
		}()
	}
	wg.Wait()
}
