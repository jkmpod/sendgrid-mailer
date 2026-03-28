package mailer

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jkmpod/sendgrid-mailer/models"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// BatchError records a failure for a specific batch during bulk sending.
type BatchError struct {
	BatchIndex int
	Err        error
}

// SendResult summarises the outcome of a bulk send operation.
// Partial success is expected — check BatchErrors for per-batch details.
type SendResult struct {
	TotalSent    int
	TotalFailed  int
	BatchErrors  []BatchError
}

// SendBatch sends a single batch of recipients. It builds the mail message,
// calls the SendGrid API, and returns the parsed response body.
func (e *Emailer) SendBatch(
	recipients []models.EmailRecipient,
	subject string,
	htmlTemplate string,
	cc []string,
	bcc []string,
) (map[string]interface{}, error) {
	fromEmail, fromName := e.GetFrom()
	from := mail.NewEmail(fromName, fromEmail)

	msg, err := BuildMail(from, subject, htmlTemplate, recipients, cc, bcc)
	if err != nil {
		return nil, fmt.Errorf("failed to build mail: %w", err)
	}

	resp, err := e.client.Send(msg)
	if err != nil {
		log.Printf("[mailer] SendBatch: API request failed: %v", err)
		return nil, fmt.Errorf("SendGrid API request failed: %w", err)
	}

	log.Printf("[mailer] SendBatch: status=%d recipients=%d", resp.StatusCode, len(recipients))

	result := make(map[string]interface{})
	result["status_code"] = resp.StatusCode
	result["headers"] = resp.Headers

	if resp.Body != "" {
		var body interface{}
		if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
			result["body"] = resp.Body
		} else {
			result["body"] = body
		}
	}

	if resp.StatusCode >= 400 {
		log.Printf("[mailer] SendBatch: ERROR status=%d body=%s", resp.StatusCode, resp.Body)
		return result, fmt.Errorf("SendGrid returned status %d", resp.StatusCode)
	}

	return result, nil
}

// SendTest sends a test email to each address in testEmails, personalised
// using data from firstRecipient (as if each test address were that person).
// The subject is prefixed with "[TEST] ". No chunking is needed — all test
// emails are sent as a single batch.
func (e *Emailer) SendTest(
	testEmails []string,
	subject string,
	htmlTemplate string,
	firstRecipient models.EmailRecipient,
	cc []string,
	bcc []string,
) (SendResult, error) {
	if len(testEmails) == 0 {
		return SendResult{}, fmt.Errorf("testEmails must not be empty")
	}

	recipients := make([]models.EmailRecipient, len(testEmails))
	for i, addr := range testEmails {
		recipients[i] = models.EmailRecipient{
			Email:        addr,
			Name:         firstRecipient.Name,
			CustomFields: firstRecipient.CustomFields,
		}
	}

	testSubject := "[TEST] " + subject
	log.Printf("[mailer] SendTest: sending to %d test addresses, subject=%q", len(testEmails), testSubject)

	_, err := e.SendBatch(recipients, testSubject, htmlTemplate, cc, bcc)
	if err != nil {
		log.Printf("[mailer] SendTest: failed: %v", err)
		return SendResult{
			TotalFailed: len(recipients),
			BatchErrors: []BatchError{{BatchIndex: 0, Err: err}},
		}, nil
	}

	log.Printf("[mailer] SendTest: success, sent=%d", len(recipients))
	return SendResult{TotalSent: len(recipients)}, nil
}

// SendBulk splits recipients into batches, sends each one, and collects
// results. It does NOT stop on the first batch error — partial success is a
// valid and expected outcome. A top-level error is returned only if something
// systemic fails (e.g. the template is unparseable). A time.Sleep of
// RateDelayMS milliseconds is inserted between batches.
func (e *Emailer) SendBulk(
	recipients []models.EmailRecipient,
	subject string,
	htmlTemplate string,
	cc []string,
	bcc []string,
) (SendResult, error) {
	chunks := ChunkRecipients(recipients, e.MaxBatchSize)
	log.Printf("[mailer] SendBulk: starting send to %d recipients in %d batches", len(recipients), len(chunks))

	var sr SendResult

	for i, chunk := range chunks {
		if i > 0 {
			time.Sleep(time.Duration(e.RateDelayMS) * time.Millisecond)
		}

		_, err := e.SendBatch(chunk, subject, htmlTemplate, cc, bcc)
		if err != nil {
			sr.TotalFailed += len(chunk)
			sr.BatchErrors = append(sr.BatchErrors, BatchError{
				BatchIndex: i,
				Err:        err,
			})
			log.Printf("[mailer] SendBulk: batch %d/%d failed: %v", i+1, len(chunks), err)
			continue
		}

		sr.TotalSent += len(chunk)
		log.Printf("[mailer] SendBulk: batch %d/%d ok, sent=%d", i+1, len(chunks), len(chunk))
	}

	log.Printf("[mailer] SendBulk: complete, totalSent=%d totalFailed=%d", sr.TotalSent, sr.TotalFailed)
	return sr, nil
}
