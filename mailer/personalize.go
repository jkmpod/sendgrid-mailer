package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	texttmpl "text/template"

	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/jkmpod/sendgrid-mailer/models"
)

// BuildMail constructs an SGMailV3 message for a single recipient. The
// htmlTemplate is parsed as Go html/template and the subject string is
// parsed as Go text/template, each executed with the recipient's data
// (keys: Email, Name, plus every CustomField key). The rendered HTML body
// is set directly as the message content — no SendGrid substitution tokens
// are used. CC and BCC addresses are added to the single Personalization.
// Optional categories (up to 10, max 255 chars each) are attached at the
// message level via m.AddCategories. Referencing an unknown field (one that
// is not present in the recipient's data) returns an error; templates must
// only reference columns that exist in the CSV.
func BuildMail(
	from *mail.Email,
	subject string,
	htmlTemplate string,
	recipient models.EmailRecipient,
	cc []string,
	bcc []string,
	categories []string,
) (*mail.SGMailV3, error) {
	bodyTmpl, err := template.New("body").Option("missingkey=error").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	subjTmpl, err := texttmpl.New("subject").Option("missingkey=error").Parse(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject template: %w", err)
	}

	data := make(map[string]string, len(recipient.CustomFields)+2)
	data["Email"] = recipient.Email
	data["Name"] = recipient.Name
	for k, v := range recipient.CustomFields {
		data[k] = v
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		return nil, fmt.Errorf("failed to execute HTML template for %s: %w", recipient.Email, err)
	}

	var subjBuf bytes.Buffer
	if err := subjTmpl.Execute(&subjBuf, data); err != nil {
		return nil, fmt.Errorf("failed to execute subject template for %s: %w", recipient.Email, err)
	}

	m := mail.NewV3Mail()
	m.SetFrom(from)
	m.Subject = subjBuf.String()

	if len(categories) > 0 {
		m.AddCategories(categories...)
	}

	p := mail.NewPersonalization()
	p.AddTos(mail.NewEmail(recipient.Name, recipient.Email))
	for _, addr := range cc {
		p.AddCCs(mail.NewEmail("", addr))
	}
	for _, addr := range bcc {
		p.AddBCCs(mail.NewEmail("", addr))
	}
	m.AddPersonalizations(p)

	m.AddContent(mail.NewContent("text/html", bodyBuf.String()))

	return m, nil
}
