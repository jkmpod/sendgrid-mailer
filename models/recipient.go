package models

// EmailRecipient represents a single email recipient loaded from a CSV file.
// It carries the recipient's address, display name, and any extra columns as
// key-value pairs for template substitution.
type EmailRecipient struct {
	Email        string
	Name         string
	CustomFields map[string]string
}
