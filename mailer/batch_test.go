package mailer

import (
	"testing"

	"github.com/jkmpod/sendgrid-mailer/models"
)

func makeRecipients(n int) []models.EmailRecipient {
	out := make([]models.EmailRecipient, n)
	for i := range out {
		out[i] = models.EmailRecipient{Email: "user@example.com", Name: "User"}
	}
	return out
}

func TestChunkRecipients(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		batchSize  int
		wantChunks int
		wantLast   int // expected length of the last chunk
	}{
		{
			name:       "empty input",
			count:      0,
			batchSize:  10,
			wantChunks: 0,
			wantLast:   0,
		},
		{
			name:       "single batch (fewer than batchSize)",
			count:      5,
			batchSize:  10,
			wantChunks: 1,
			wantLast:   5,
		},
		{
			name:       "multiple batches with remainder",
			count:      25,
			batchSize:  10,
			wantChunks: 3,
			wantLast:   5,
		},
		{
			name:       "exact boundary (no remainder)",
			count:      20,
			batchSize:  10,
			wantChunks: 2,
			wantLast:   10,
		},
		{
			name:       "batch size of 1",
			count:      3,
			batchSize:  1,
			wantChunks: 3,
			wantLast:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recipients := makeRecipients(tt.count)
			chunks := ChunkRecipients(recipients, tt.batchSize)

			if len(chunks) != tt.wantChunks {
				t.Fatalf("got %d chunks, want %d", len(chunks), tt.wantChunks)
			}
			if tt.wantChunks > 0 {
				last := chunks[len(chunks)-1]
				if len(last) != tt.wantLast {
					t.Errorf("last chunk has %d items, want %d", len(last), tt.wantLast)
				}
			}

			// Verify total count across all chunks equals input count.
			total := 0
			for _, c := range chunks {
				total += len(c)
			}
			if total != tt.count {
				t.Errorf("total recipients across chunks = %d, want %d", total, tt.count)
			}
		})
	}
}
