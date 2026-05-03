package models

// EmailRecipient represents a single email recipient loaded from a CSV file.
// It carries the recipient's address, display name, and any extra columns as
// key-value pairs for template substitution.
type EmailRecipient struct {
	// Email is the recipient's email address (required, non-empty).
	Email string
	// Name is the recipient's display name (may be empty).
	Name string
	// CustomFields holds every non-email/non-name CSV column keyed by header name, used for template substitution.
	CustomFields map[string]string
}
