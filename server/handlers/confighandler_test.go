package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/mailer"
)

func TestHandleConfig(t *testing.T) {
	tests := []struct {
		name            string
		testMode        bool
		wantTestMode    bool
		setSubject      string // value to set before calling handler
		wantLastSubject string
	}{
		{
			name:            "test mode enabled",
			testMode:        true,
			wantTestMode:    true,
			setSubject:      "",
			wantLastSubject: "",
		},
		{
			name:            "test mode disabled",
			testMode:        false,
			wantTestMode:    false,
			setSubject:      "",
			wantLastSubject: "",
		},
		{
			name:            "empty lastSubject before any send",
			testMode:        false,
			wantTestMode:    false,
			setSubject:      "",
			wantLastSubject: "",
		},
		{
			name:            "lastSubject populated after SetLastSubject",
			testMode:        false,
			wantTestMode:    false,
			setSubject:      "Welcome Newsletter",
			wantLastSubject: "Welcome Newsletter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLastSubject(tt.setSubject)
			ResetSendLog()
			ResetRuntimeConfig()

			cfg := &config.Config{
				TestMode: tt.testMode,
			}

			handler := HandleConfig(cfg)
			req := httptest.NewRequest("GET", "/config", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			got, ok := resp["testMode"].(bool)
			if !ok {
				t.Fatalf("expected testMode bool in response, got: %v", resp)
			}
			if got != tt.wantTestMode {
				t.Errorf("testMode = %v, want %v", got, tt.wantTestMode)
			}

			gotSubject, ok := resp["lastSubject"].(string)
			if !ok {
				t.Fatalf("expected lastSubject string in response, got: %v", resp)
			}
			if gotSubject != tt.wantLastSubject {
				t.Errorf("lastSubject = %q, want %q", gotSubject, tt.wantLastSubject)
			}

			// sendLog should always be present as an array.
			gotLog, ok := resp["sendLog"].([]interface{})
			if !ok {
				t.Fatalf("expected sendLog array in response, got: %v", resp["sendLog"])
			}
			if len(gotLog) != 0 {
				t.Errorf("sendLog len = %d, want 0 (reset before each case)", len(gotLog))
			}
		})
	}
}

func TestHandleConfig_WithSendLog(t *testing.T) {
	ResetSendLog()
	SetLastSubject("")

	AppendSendLog(SendLogEntry{
		Subject:     "Test Email",
		TotalSent:   3,
		TotalFailed: 0,
		TestMode:    false,
	})

	cfg := &config.Config{}
	handler := HandleConfig(cfg)
	req := httptest.NewRequest("GET", "/config", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	gotLog, ok := resp["sendLog"].([]interface{})
	if !ok {
		t.Fatalf("expected sendLog array, got: %v", resp["sendLog"])
	}
	if len(gotLog) != 1 {
		t.Fatalf("sendLog len = %d, want 1", len(gotLog))
	}

	entry, ok := gotLog[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected sendLog entry to be object, got: %v", gotLog[0])
	}
	if entry["subject"] != "Test Email" {
		t.Errorf("entry.subject = %q, want %q", entry["subject"], "Test Email")
	}
	if int(entry["totalSent"].(float64)) != 3 {
		t.Errorf("entry.totalSent = %v, want 3", entry["totalSent"])
	}
}

func TestHandleConfigUpdate(t *testing.T) {
	cfg := &config.Config{
		APIKey:    "SG.test",
		FromEmail: "default@example.com",
		FromName:  "Default",
		TestMode:  true,
	}
	e := mailer.NewEmailer(cfg)

	tests := []struct {
		name         string
		body         string
		wantStatus   int
		wantTestMode bool
		wantFrom     string
	}{
		{
			name:         "toggle test mode off",
			body:         `{"testMode": false}`,
			wantStatus:   http.StatusOK,
			wantTestMode: false,
			wantFrom:     "default@example.com",
		},
		{
			name:         "update from email",
			body:         `{"fromEmail": "new@example.com"}`,
			wantStatus:   http.StatusOK,
			wantTestMode: false, // retains previous override
			wantFrom:     "new@example.com",
		},
		{
			name:         "update test emails",
			body:         `{"testEmails": ["a@x.com", "b@x.com"]}`,
			wantStatus:   http.StatusOK,
			wantTestMode: false,
			wantFrom:     "new@example.com",
		},
		{
			name:       "invalid JSON returns 400",
			body:       `{bad`,
			wantStatus: http.StatusBadRequest,
		},
	}

	ResetRuntimeConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := HandleConfigUpdate(e, cfg)
			req := httptest.NewRequest("POST", "/config", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if got := resp["testMode"].(bool); got != tt.wantTestMode {
				t.Errorf("testMode = %v, want %v", got, tt.wantTestMode)
			}
			if tt.wantFrom != "" {
				if got := resp["fromEmail"].(string); got != tt.wantFrom {
					t.Errorf("fromEmail = %q, want %q", got, tt.wantFrom)
				}
			}
		})
	}
}
