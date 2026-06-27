package mailer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/jkmpod/sendgrid-mailer/models"
)

// RecipientError records a send failure for a single recipient.
type RecipientError struct {
	// Email is the recipient address that failed.
	Email string
	// Err is the underlying error returned by SendOne for this recipient.
	Err error
}

// SendResult summarises the outcome of a bulk send operation.
// Partial success is expected — check Failures for per-recipient details.
type SendResult struct {
	// TotalSent is the count of recipients the SendGrid API accepted.
	TotalSent int
	// TotalFailed is the count of recipients the SendGrid API rejected.
	TotalFailed int
	// Failures lists the per-recipient failures encountered during the send.
	Failures []RecipientError
}

// ValidateSend checks that the subject and HTML template parse and render
// for the given recipient, returning a descriptive error if not. It performs
// no network I/O — use it to fail fast before a bulk send.
func (e *Emailer) ValidateSend(recipient models.EmailRecipient, subject, htmlTemplate string, cc, bcc, categories []string) error {
	fromEmail, fromName := e.GetFrom()
	from := mail.NewEmail(fromName, fromEmail)
	_, err := BuildMail(from, subject, htmlTemplate, recipient, cc, bcc, categories)
	return err
}

// SendOne sends a single email to one recipient. It calls SendOneWithContext
// with context.Background().
func (e *Emailer) SendOne(
	recipient models.EmailRecipient,
	subject string,
	htmlTemplate string,
	cc []string,
	bcc []string,
	categories []string,
) (map[string]interface{}, error) {
	return e.SendOneWithContext(context.Background(), recipient, subject, htmlTemplate, cc, bcc, categories)
}

// SendOneWithContext sends a single email to one recipient, respecting the
// provided context for cancellation. It builds the mail message via BuildMail,
// calls the SendGrid API, and returns the parsed response body. Optional
// categories are forwarded to BuildMail and attached at the message level.
func (e *Emailer) SendOneWithContext(
	ctx context.Context,
	recipient models.EmailRecipient,
	subject string,
	htmlTemplate string,
	cc []string,
	bcc []string,
	categories []string,
) (map[string]interface{}, error) {
	fromEmail, fromName := e.GetFrom()
	from := mail.NewEmail(fromName, fromEmail)

	msg, err := BuildMail(from, subject, htmlTemplate, recipient, cc, bcc, categories)
	if err != nil {
		return nil, fmt.Errorf("failed to build mail: %w", err)
	}

	client := func() *sendgrid.Client {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.client
	}()

	attempts := e.RetryMaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		var attemptCtx context.Context
		var cancel context.CancelFunc
		if e.TimeoutMS > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, time.Duration(e.TimeoutMS)*time.Millisecond)
		} else {
			attemptCtx, cancel = context.WithCancel(ctx)
		}
		resp, err := client.SendWithContext(attemptCtx, msg)
		cancel()

		var statusCode int
		if err == nil {
			statusCode = resp.StatusCode
		}

		// Success path: parse and return.
		if err == nil && statusCode < 400 {
			log.Printf("[mailer] SendOne: status=%d recipient=%s", resp.StatusCode, recipient.Email)
			result := make(map[string]interface{})
			result["status_code"] = resp.StatusCode
			result["headers"] = resp.Headers
			if resp.Body != "" {
				var body interface{}
				if jsonErr := json.Unmarshal([]byte(resp.Body), &body); jsonErr != nil {
					result["body"] = resp.Body
				} else {
					result["body"] = body
				}
			}
			return result, nil
		}

		// Retry if this is not the last attempt and the failure is transient.
		if attempt < attempts && isTransient(statusCode, err) {
			log.Printf("[mailer] SendOne: transient error on attempt %d/%d for %s (status %d, err: %v); retrying", attempt, attempts, recipient.Email, statusCode, err)

			// Wait with backoff, but abort if parent context is cancelled.
			timer := time.NewTimer(backoff(attempt, e.RetryBackoffMS))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}

		// Final failure: permanent error or last attempt.
		if err != nil {
			log.Printf("[mailer] SendOne: API request failed for %s: %v", recipient.Email, err)
			return nil, fmt.Errorf("SendGrid API request failed: %w", err)
		}
		log.Printf("[mailer] SendOne: status=%d recipient=%s", resp.StatusCode, recipient.Email)
		result := make(map[string]interface{})
		result["status_code"] = resp.StatusCode
		result["headers"] = resp.Headers
		if resp.Body != "" {
			var body interface{}
			if jsonErr := json.Unmarshal([]byte(resp.Body), &body); jsonErr != nil {
				result["body"] = resp.Body
			} else {
				result["body"] = body
			}
		}
		log.Printf("[mailer] SendOne: ERROR status=%d recipient=%s body=%s", resp.StatusCode, recipient.Email, resp.Body)
		return result, fmt.Errorf("SendGrid returned status %d", resp.StatusCode)
	}

	// Unreachable: the loop body always returns on the last attempt.
	return nil, fmt.Errorf("SendGrid API request failed: all retries exhausted")
}

// SendTest sends a test email to each address in testEmails. It calls
// SendTestWithContext with context.Background().
func (e *Emailer) SendTest(
	testEmails []string,
	subject string,
	htmlTemplate string,
	firstRecipient models.EmailRecipient,
	cc []string,
	bcc []string,
	categories []string,
) (SendResult, error) {
	return e.SendTestWithContext(context.Background(), testEmails, subject, htmlTemplate, firstRecipient, cc, bcc, categories)
}

// SendTestWithContext sends a test email to each address in testEmails,
// personalised using data from firstRecipient (as if each test address were
// that person). The subject is prefixed with "[TEST] ". One SendGrid API call
// is made per test address, separated by RateDelayMS. Optional categories are
// forwarded to each SendOne call and attached at the message level. It
// respects the provided context for cancellation.
func (e *Emailer) SendTestWithContext(
	ctx context.Context,
	testEmails []string,
	subject string,
	htmlTemplate string,
	firstRecipient models.EmailRecipient,
	cc []string,
	bcc []string,
	categories []string,
) (SendResult, error) {
	if len(testEmails) == 0 {
		return SendResult{}, fmt.Errorf("testEmails must not be empty")
	}

	testSubject := "[TEST] " + subject
	log.Printf("[mailer] SendTest: sending to %d test addresses, subject=%q", len(testEmails), testSubject)

	var sr SendResult
	for i, addr := range testEmails {
		if i > 0 {
			timer := time.NewTimer(time.Duration(e.RateDelayMS) * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return sr, ctx.Err()
			case <-timer.C:
			}
		}

		r := models.EmailRecipient{
			Email:        addr,
			Name:         firstRecipient.Name,
			CustomFields: firstRecipient.CustomFields,
		}

		_, err := e.SendOneWithContext(ctx, r, testSubject, htmlTemplate, cc, bcc, categories)
		if err != nil {
			log.Printf("[mailer] SendTest: failed for %s: %v", addr, err)
			sr.TotalFailed++
			sr.Failures = append(sr.Failures, RecipientError{Email: addr, Err: err})
			continue
		}
		sr.TotalSent++
	}

	log.Printf("[mailer] SendTest: complete, sent=%d failed=%d", sr.TotalSent, sr.TotalFailed)
	return sr, nil
}

// SendBulk iterates over recipients and sends one email per recipient. It
// calls SendBulkWithContext with context.Background().
func (e *Emailer) SendBulk(
	recipients []models.EmailRecipient,
	subject string,
	htmlTemplate string,
	cc []string,
	bcc []string,
	categories []string,
) (SendResult, error) {
	return e.SendBulkWithContext(context.Background(), recipients, subject, htmlTemplate, cc, bcc, categories)
}

// SendBulkWithContext iterates over recipients and sends one email per
// recipient via SendOneWithContext. It does NOT stop on a per-recipient error
// — partial success is a valid and expected outcome. A delay of RateDelayMS
// milliseconds is inserted between sends (skipped before the first). It
// respects the provided context for cancellation.
func (e *Emailer) SendBulkWithContext(
	ctx context.Context,
	recipients []models.EmailRecipient,
	subject string,
	htmlTemplate string,
	cc []string,
	bcc []string,
	categories []string,
) (SendResult, error) {
	log.Printf("[mailer] SendBulk: starting send to %d recipients", len(recipients))

	var sr SendResult

	for i, r := range recipients {
		if i > 0 {
			timer := time.NewTimer(time.Duration(e.RateDelayMS) * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return sr, ctx.Err()
			case <-timer.C:
			}
		}

		_, err := e.SendOneWithContext(ctx, r, subject, htmlTemplate, cc, bcc, categories)
		if err != nil {
			sr.TotalFailed++
			sr.Failures = append(sr.Failures, RecipientError{Email: r.Email, Err: err})
			log.Printf("[mailer] SendBulk: recipient %d/%d (%s) failed: %v", i+1, len(recipients), r.Email, err)
			continue
		}

		sr.TotalSent++
		log.Printf("[mailer] SendBulk: recipient %d/%d (%s) ok", i+1, len(recipients), r.Email)
	}

	log.Printf("[mailer] SendBulk: complete, totalSent=%d totalFailed=%d", sr.TotalSent, sr.TotalFailed)
	return sr, nil
}
