package mailer

import (
	"strings"
	"testing"

	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/jkmpod/sendgrid-mailer/models"
)

func TestBuildMail_Categories(t *testing.T) {
	from := mail.NewEmail("Test Sender", "sender@example.com")
	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}
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
			m, err := BuildMail(from, "Test Subject", tmpl, recipient, nil, nil, tt.categories)
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
	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}

	_, err := BuildMail(from, "Subject", "{{.Unclosed", recipient, nil, nil, nil)
	if err == nil {
		t.Error("expected error for invalid template, got nil")
	}
}

func TestBuildMail_UnknownFieldReturnsError(t *testing.T) {
	// missingkey=error: referencing a column not present in the recipient's
	// data must return an error rather than rendering "<no value>".
	from := mail.NewEmail("Test Sender", "sender@example.com")
	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}

	tests := []struct {
		name        string
		subject     string
		htmlTmpl    string
		wantErrSub  string
	}{
		{
			name:       "unknown field in body",
			subject:    "Hello",
			htmlTmpl:   "<p>{{.Nope}}</p>",
			wantErrSub: "Nope",
		},
		{
			name:       "unknown field in subject",
			subject:    "Hi {{.Typo}}",
			htmlTmpl:   "<p>body</p>",
			wantErrSub: "Typo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildMail(from, tt.subject, tt.htmlTmpl, recipient, nil, nil, nil)
			if err == nil {
				t.Fatal("expected error for unknown template field, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestBuildMail_DirectBodyContent(t *testing.T) {
	from := mail.NewEmail("Test Sender", "sender@example.com")
	recipient := models.EmailRecipient{
		Email: "alice@example.com",
		Name:  "Alice",
		CustomFields: map[string]string{
			"company": "Acme",
		},
	}
	tmpl := "<p>Hello {{.Name}}, you work at {{.company}}.</p>"

	m, err := BuildMail(from, "Hi {{.Name}}", tmpl, recipient, nil, nil, nil)
	if err != nil {
		t.Fatalf("BuildMail returned unexpected error: %v", err)
	}

	// (a) Body content equals directly-rendered HTML and contains no substitution token.
	if len(m.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	want := "<p>Hello Alice, you work at Acme.</p>"
	got := m.Content[0].Value
	if got != want {
		t.Errorf("body content = %q, want %q", got, want)
	}
	if strings.Contains(got, "{{html_body}}") {
		t.Errorf("body content contains substitution token {{html_body}}: %q", got)
	}

	// (b) Subject is rendered per-recipient.
	if m.Subject != "Hi Alice" {
		t.Errorf("subject = %q, want %q", m.Subject, "Hi Alice")
	}
}

func TestBuildMail_CCAndBCCExactlyOnce(t *testing.T) {
	from := mail.NewEmail("Test Sender", "sender@example.com")
	recipient := models.EmailRecipient{Email: "alice@example.com", Name: "Alice"}

	m, err := BuildMail(from, "Subject", "<p>Hi</p>", recipient,
		[]string{"cc@example.com"},
		[]string{"bcc@example.com"},
		nil,
	)
	if err != nil {
		t.Fatalf("BuildMail returned unexpected error: %v", err)
	}

	if len(m.Personalizations) != 1 {
		t.Fatalf("expected 1 personalisation, got %d", len(m.Personalizations))
	}
	p := m.Personalizations[0]

	// (b) CC appears exactly once.
	if len(p.CC) != 1 {
		t.Errorf("CC count = %d, want 1; got %v", len(p.CC), p.CC)
	} else if p.CC[0].Address != "cc@example.com" {
		t.Errorf("CC address = %q, want %q", p.CC[0].Address, "cc@example.com")
	}

	// (b) BCC appears exactly once.
	if len(p.BCC) != 1 {
		t.Errorf("BCC count = %d, want 1; got %v", len(p.BCC), p.BCC)
	} else if p.BCC[0].Address != "bcc@example.com" {
		t.Errorf("BCC address = %q, want %q", p.BCC[0].Address, "bcc@example.com")
	}
}

func TestBuildMail_SubjectPersonalised(t *testing.T) {
	// (c) Subject template is personalised per recipient.
	from := mail.NewEmail("Sender", "sender@example.com")

	tests := []struct {
		name        string
		subjectTmpl string
		recipient   models.EmailRecipient
		wantSubject string
	}{
		{
			name:        "name in subject",
			subjectTmpl: "Hi {{.Name}}",
			recipient:   models.EmailRecipient{Email: "alice@example.com", Name: "Alice"},
			wantSubject: "Hi Alice",
		},
		{
			name:        "custom field in subject",
			subjectTmpl: "Welcome to {{.company}}",
			recipient: models.EmailRecipient{
				Email:        "bob@example.com",
				Name:         "Bob",
				CustomFields: map[string]string{"company": "Widgets Inc"},
			},
			wantSubject: "Welcome to Widgets Inc",
		},
		{
			name:        "plain subject unchanged",
			subjectTmpl: "No template here",
			recipient:   models.EmailRecipient{Email: "carol@example.com", Name: "Carol"},
			wantSubject: "No template here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := BuildMail(from, tt.subjectTmpl, "<p>body</p>", tt.recipient, nil, nil, nil)
			if err != nil {
				t.Fatalf("BuildMail error: %v", err)
			}
			if m.Subject != tt.wantSubject {
				t.Errorf("subject = %q, want %q", m.Subject, tt.wantSubject)
			}
		})
	}
}

func TestBuildMail_ColumnWithSpaceUsesIndexSyntax(t *testing.T) {
	// (d) A column with a space in its name is accessible via {{index . "First Name"}}.
	from := mail.NewEmail("Sender", "sender@example.com")
	recipient := models.EmailRecipient{
		Email:        "dave@example.com",
		Name:         "Dave",
		CustomFields: map[string]string{"First Name": "David"},
	}
	tmpl := `<p>Hello {{index . "First Name"}}</p>`

	m, err := BuildMail(from, "Subject", tmpl, recipient, nil, nil, nil)
	if err != nil {
		t.Fatalf("BuildMail error: %v", err)
	}
	want := "<p>Hello David</p>"
	if m.Content[0].Value != want {
		t.Errorf("body = %q, want %q", m.Content[0].Value, want)
	}
}
