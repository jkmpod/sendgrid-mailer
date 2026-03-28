package handlers

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jkmpod/sendgrid-mailer/config"
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

func TestSendLog(t *testing.T) {
	ResetSendLog()

	t.Run("empty log returns empty slice", func(t *testing.T) {
		got := GetSendLog()
		if len(got) != 0 {
			t.Errorf("GetSendLog() len = %d, want 0", len(got))
		}
	})

	t.Run("append and retrieve entries", func(t *testing.T) {
		ResetSendLog()
		now := time.Now()
		AppendSendLog(SendLogEntry{
			Time:        now,
			Subject:     "Hello",
			TotalSent:   5,
			TotalFailed: 0,
			TestMode:    false,
		})
		AppendSendLog(SendLogEntry{
			Time:        now,
			Subject:     "[TEST] Hello",
			TotalSent:   2,
			TotalFailed: 1,
			TestMode:    true,
		})

		got := GetSendLog()
		if len(got) != 2 {
			t.Fatalf("GetSendLog() len = %d, want 2", len(got))
		}
		if got[0].Subject != "Hello" {
			t.Errorf("entry[0].Subject = %q, want %q", got[0].Subject, "Hello")
		}
		if got[1].TotalFailed != 1 {
			t.Errorf("entry[1].TotalFailed = %d, want 1", got[1].TotalFailed)
		}
	})

	t.Run("capped at 50 entries", func(t *testing.T) {
		ResetSendLog()
		for i := 0; i < 60; i++ {
			AppendSendLog(SendLogEntry{
				Time:    time.Now(),
				Subject: fmt.Sprintf("Subject %d", i),
			})
		}
		got := GetSendLog()
		if len(got) != 50 {
			t.Errorf("GetSendLog() len = %d, want 50", len(got))
		}
		// Oldest entries (0-9) should be dropped; first entry should be "Subject 10".
		if got[0].Subject != "Subject 10" {
			t.Errorf("first entry = %q, want %q", got[0].Subject, "Subject 10")
		}
	})

	t.Run("returns a copy not a reference", func(t *testing.T) {
		ResetSendLog()
		AppendSendLog(SendLogEntry{Time: time.Now(), Subject: "original"})
		got := GetSendLog()
		got[0].Subject = "mutated"
		// The internal log should not be affected.
		got2 := GetSendLog()
		if got2[0].Subject != "original" {
			t.Errorf("internal log was mutated: got %q, want %q", got2[0].Subject, "original")
		}
	})
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

func TestRuntimeConfig(t *testing.T) {
	cfg := &config.Config{
		TestMode:  true,
		TestEmails: []string{"default@example.com"},
		FromEmail: "default@example.com",
		FromName:  "Default Sender",
	}

	t.Run("defaults when no override set", func(t *testing.T) {
		ResetRuntimeConfig()
		if got := EffectiveTestMode(cfg); got != true {
			t.Errorf("EffectiveTestMode = %v, want true", got)
		}
		if got := EffectiveTestEmails(cfg); len(got) != 1 || got[0] != "default@example.com" {
			t.Errorf("EffectiveTestEmails = %v, want [default@example.com]", got)
		}
		if got := EffectiveFromEmail(cfg); got != "default@example.com" {
			t.Errorf("EffectiveFromEmail = %q, want %q", got, "default@example.com")
		}
		if got := EffectiveFromName(cfg); got != "Default Sender" {
			t.Errorf("EffectiveFromName = %q, want %q", got, "Default Sender")
		}
	})

	t.Run("overrides take precedence", func(t *testing.T) {
		ResetRuntimeConfig()
		SetRuntimeTestMode(false)
		SetRuntimeTestEmails([]string{"override@x.com"})
		SetRuntimeFromEmail("override@x.com")
		SetRuntimeFromName("Override Sender")

		if got := EffectiveTestMode(cfg); got != false {
			t.Errorf("EffectiveTestMode = %v, want false", got)
		}
		if got := EffectiveTestEmails(cfg); len(got) != 1 || got[0] != "override@x.com" {
			t.Errorf("EffectiveTestEmails = %v, want [override@x.com]", got)
		}
		if got := EffectiveFromEmail(cfg); got != "override@x.com" {
			t.Errorf("EffectiveFromEmail = %q, want %q", got, "override@x.com")
		}
		if got := EffectiveFromName(cfg); got != "Override Sender" {
			t.Errorf("EffectiveFromName = %q, want %q", got, "Override Sender")
		}
	})

	t.Run("reset clears all overrides", func(t *testing.T) {
		SetRuntimeTestMode(false)
		SetRuntimeTestEmails([]string{"x@x.com"})
		SetRuntimeFromEmail("x@x.com")
		SetRuntimeFromName("X")
		ResetRuntimeConfig()

		if got := GetRuntimeTestMode(); got != nil {
			t.Errorf("GetRuntimeTestMode = %v, want nil", got)
		}
		if got := GetRuntimeTestEmails(); got != nil {
			t.Errorf("GetRuntimeTestEmails = %v, want nil", got)
		}
		if got := GetRuntimeFromEmail(); got != "" {
			t.Errorf("GetRuntimeFromEmail = %q, want empty", got)
		}
		if got := GetRuntimeFromName(); got != "" {
			t.Errorf("GetRuntimeFromName = %q, want empty", got)
		}
	})
}
