package mailer

import (
	"testing"

	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/jkmpod/sendgrid-mailer/models"
)

func TestBuildMail_Categories(t *testing.T) {
	from := mail.NewEmail("Test Sender", "sender@example.com")
	recipients := []models.EmailRecipient{
		{Email: "alice@example.com", Name: "Alice"},
	}
	tmpl := "<p>Hello {{.Name}}</p>"

	tests := []struct {
		name           string
		categories     []string
		wantCategories []string
	}{
		{
			name:           "nil categories attaches none",
			categories:     nil,
			wantCategories: nil,
		},
		{
			name:           "empty slice attaches none",
			categories:     []string{},
			wantCategories: nil,
		},
		{
			name:           "single category is attached",
			categories:     []string{"newsletter"},
			wantCategories: []string{"newsletter"},
		},
		{
			name:           "multiple categories are all attached",
			categories:     []string{"newsletter", "march-2026", "promo"},
			wantCategories: []string{"newsletter", "march-2026", "promo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := BuildMail(from, "Test Subject", tmpl, recipients, nil, nil, tt.categories)
			if err != nil {
				t.Fatalf("BuildMail returned unexpected error: %v", err)
			}

			if len(tt.wantCategories) == 0 {
				if len(m.Categories) != 0 {
					t.Errorf("expected no categories, got %v", m.Categories)
				}
				return
			}

			if len(m.Categories) != len(tt.wantCategories) {
				t.Fatalf("len(m.Categories) = %d, want %d; got %v", len(m.Categories), len(tt.wantCategories), m.Categories)
			}
			for i, want := range tt.wantCategories {
				if m.Categories[i] != want {
					t.Errorf("m.Categories[%d] = %q, want %q", i, m.Categories[i], want)
				}
			}
		})
	}
}

func TestBuildMail_InvalidTemplate(t *testing.T) {
	from := mail.NewEmail("Test Sender", "sender@example.com")
	recipients := []models.EmailRecipient{{Email: "alice@example.com", Name: "Alice"}}

	_, err := BuildMail(from, "Subject", "{{.Unclosed", recipients, nil, nil, nil)
	if err == nil {
		t.Error("expected error for invalid template, got nil")
	}
}
