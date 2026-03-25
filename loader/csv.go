package loader

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jkmpod/sendgrid-mailer/models"
)

// LoadFromCSV reads a CSV file and returns a slice of EmailRecipient values.
// The first row is treated as a header. The "email" and "name" columns
// (case-insensitive) map to the struct fields; all other columns become
// entries in CustomFields. Rows with an empty email are skipped with a warning.
func LoadFromCSV(filePath string) ([]models.EmailRecipient, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open CSV file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Read all rows at once; returns an error if the CSV is malformed
	// (e.g., inconsistent field counts).
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unable to parse CSV file: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV file is empty, expected at least a header row")
	}

	header := records[0]

	// Build a map of lowercase column name → column index.
	emailIdx := -1
	nameIdx := -1
	colMap := make(map[int]string, len(header)) // index → original header name

	for i, col := range header {
		lower := strings.ToLower(strings.TrimSpace(col))
		switch lower {
		case "email":
			emailIdx = i
		case "name":
			nameIdx = i
		default:
			colMap[i] = strings.TrimSpace(col)
		}
	}

	if emailIdx == -1 {
		return nil, fmt.Errorf("CSV file is missing a required 'email' column")
	}

	var recipients []models.EmailRecipient

	for rowNum, row := range records[1:] {
		email := strings.TrimSpace(row[emailIdx])
		if email == "" {
			log.Printf("warning: skipping row %d — email is empty", rowNum+2)
			continue
		}

		name := ""
		if nameIdx != -1 {
			name = strings.TrimSpace(row[nameIdx])
		}

		custom := make(map[string]string, len(colMap))
		for idx, key := range colMap {
			custom[key] = strings.TrimSpace(row[idx])
		}

		recipients = append(recipients, models.EmailRecipient{
			Email:        email,
			Name:         name,
			CustomFields: custom,
		})
	}

	return recipients, nil
}
