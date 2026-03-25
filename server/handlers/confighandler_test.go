package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jkmpod/sendgrid-mailer/config"
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
		})
	}
}
