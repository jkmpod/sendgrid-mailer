package mailer

import "github.com/jkmpod/sendgrid-mailer/models"

// ChunkRecipients splits a slice of recipients into batches of at most
// batchSize elements. It returns a slice of slices rather than a channel
// because the full list is already in memory, the caller needs random access
// to report per-batch errors, and a simple slice is easier to test and reason
// about than a channel.
func ChunkRecipients(recipients []models.EmailRecipient, batchSize int) [][]models.EmailRecipient {
	if batchSize <= 0 {
		batchSize = 1
	}

	var chunks [][]models.EmailRecipient
	for i := 0; i < len(recipients); i += batchSize {
		end := i + batchSize
		if end > len(recipients) {
			end = len(recipients)
		}
		chunks = append(chunks, recipients[i:end])
	}
	return chunks
}
