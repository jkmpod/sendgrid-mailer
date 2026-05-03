package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp creates a temporary CSV file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// generateRows produces a valid CSV string with a header row and n data rows.
// Each row has an email, name, and company column.
func generateRows(n int) string {
	var b strings.Builder
	b.WriteString("email,name,company\n")
	for i := 1; i <= n; i++ {
		// strings.Builder never returns an error from Write methods.
		_, _ = fmt.Fprintf(&b, "user%d@example.com,User %d,Company%d\n", i, i, i)
	}
	return b.String()
}

func TestLoadFromCSV(t *testing.T) {
	tests := []struct {
		name       string
		csv        string
		wantCount  int
		wantErr    string // substring; "" means no error
		checkFirst func(t *testing.T, email, nameField string, custom map[string]string)
	}{
		{
			name:      "valid CSV with custom fields",
			csv:       "email,name,company\nalice@ex.com,Alice,Acme\nbob@ex.com,Bob,Globex\n",
			wantCount: 2,
			checkFirst: func(t *testing.T, email, nameField string, custom map[string]string) {
				if email != "alice@ex.com" {
					t.Errorf("Email = %q, want %q", email, "alice@ex.com")
				}
				if nameField != "Alice" {
					t.Errorf("Name = %q, want %q", nameField, "Alice")
				}
				if custom["company"] != "Acme" {
					t.Errorf("CustomFields[company] = %q, want %q", custom["company"], "Acme")
				}
			},
		},
		{
			name:      "case-insensitive headers",
			csv:       "Email,NAME,City\na@b.com,A,NY\n",
			wantCount: 1,
			checkFirst: func(t *testing.T, email, nameField string, custom map[string]string) {
				if email != "a@b.com" {
					t.Errorf("Email = %q, want %q", email, "a@b.com")
				}
				if nameField != "A" {
					t.Errorf("Name = %q, want %q", nameField, "A")
				}
				if custom["City"] != "NY" {
					t.Errorf("CustomFields[City] = %q, want %q", custom["City"], "NY")
				}
			},
		},
		{
			name:      "missing email column",
			csv:       "name,company\nAlice,Acme\n",
			wantErr:   "missing a required 'email' column",
			wantCount: 0,
		},
		{
			name:      "empty file",
			csv:       "",
			wantErr:   "empty",
			wantCount: 0,
		},
		{
			name:      "skip rows with empty email",
			csv:       "email,name\nalice@ex.com,Alice\n,Bob\ncharlie@ex.com,Charlie\n",
			wantCount: 2,
			checkFirst: func(t *testing.T, email, nameField string, custom map[string]string) {
				if email != "alice@ex.com" {
					t.Errorf("Email = %q, want %q", email, "alice@ex.com")
				}
			},
		},
		{
			name:    "malformed CSV (unmatched quote)",
			csv:     "email,name\n\"alice@ex.com,Alice\n",
			wantErr: "unable to parse CSV",
		},
		{
			name:      "header only, no data rows",
			csv:       "email,name\n",
			wantCount: 0,
		},
		{
			name:      "more rows than one batch size",
			csv:       generateRows(1001), // you will need a helper that creates N rows
			wantCount: 1001,
			wantErr:   "",
		},
		{
			name:      "no name column",
			csv:       "email,company\nalice@ex.com,Acme\n",
			wantCount: 1,
			checkFirst: func(t *testing.T, email, nameField string, custom map[string]string) {
				if nameField != "" {
					t.Errorf("Name = %q, want empty", nameField)
				}
				if custom["company"] != "Acme" {
					t.Errorf("CustomFields[company] = %q, want %q", custom["company"], "Acme")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, tt.csv)
			recipients, err := LoadFromCSV(path)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(recipients) != tt.wantCount {
				t.Fatalf("got %d recipients, want %d", len(recipients), tt.wantCount)
			}
			if tt.checkFirst != nil && len(recipients) > 0 {
				r := recipients[0]
				tt.checkFirst(t, r.Email, r.Name, r.CustomFields)
			}
		})
	}
}

func TestLoadFromCSV_FileNotFound(t *testing.T) {
	_, err := LoadFromCSV("/nonexistent/path/test.csv")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "unable to open CSV file") {
		t.Fatalf("expected 'unable to open CSV file' error, got: %v", err)
	}
}
