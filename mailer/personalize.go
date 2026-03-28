package mailer

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/jkmpod/sendgrid-mailer/models"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// BuildMail constructs an SGMailV3 message with one Personalization per
// recipient. The htmlTemplate string is parsed as a Go text/template and
// executed once for each recipient. Template data includes "Email", "Name",
// and every key from recipient.CustomFields.
func BuildMail(
	from *mail.Email,
	subject string,
	htmlTemplate string,
	recipients []models.EmailRecipient,
	cc []string,
	bcc []string,
) (*mail.SGMailV3, error) {
	tmpl, err := template.New("email").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	m := mail.NewV3Mail()
	m.SetFrom(from)
	m.Subject = subject

	for i, r := range recipients {
		data := make(map[string]string, len(r.CustomFields)+2)
		data["Email"] = r.Email
		data["Name"] = r.Name
		for k, v := range r.CustomFields {
			data[k] = v
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to execute template for recipient %d (%s): %w", i, r.Email, err)
		}

		p := mail.NewPersonalization()
		p.AddTos(mail.NewEmail(r.Name, r.Email))
		for _, addr := range cc {
			p.AddCCs(mail.NewEmail("", addr))
		}
		for _, addr := range bcc {
			p.AddBCCs(mail.NewEmail("", addr))
		}
		m.AddPersonalizations(p)

		// Each recipient gets their own rendered HTML; we store it as content
		// only once (SendGrid uses the same content for all personalizations).
		// For per-recipient content, we use substitution tags instead.
		// Here we render once and set substitution so each personalization
		// can have unique body text via the rendered HTML.
		p.SetSubstitution("{{html_body}}", buf.String())
	}

	// Set the content with a substitution placeholder that each
	// personalization replaces with its own rendered HTML.
	m.AddContent(mail.NewContent("text/html", "{{html_body}}"))

	return m, nil
}
