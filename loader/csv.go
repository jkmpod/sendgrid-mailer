package loader

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jkmpod/sendgrid-mailer/models"
)

// LoadFromCSV reads a CSV file and returns a slice of EmailRecipient values,
// a list of human-readable warnings, and any error. The first row is treated
// as a header. The "email" and "name" columns (case-insensitive) map to the
// struct fields; all other column names are normalised to lowercase before
// being stored in CustomFields, so templates must use the lowercase form
// (e.g. {{.teamname}} for a header "TeamName"). A warning is appended for
// each column whose name was changed by normalisation, and for any two
// columns that collide after normalisation. Rows with an empty email are
// skipped with a log warning.
func LoadFromCSV(filePath string) ([]models.EmailRecipient, []string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open CSV file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Read all rows at once; returns an error if the CSV is malformed
	// (e.g., inconsistent field counts).
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse CSV file: %w", err)
	}

	if len(records) < 1 {
		return nil, nil, fmt.Errorf("CSV file is empty, expected at least a header row")
	}

	header := records[0]

	emailIdx := -1
	nameIdx := -1
	colMap := make(map[int]string, len(header)) // index → normalised (lowercase) name

	var warnings []string
	seenNorm := make(map[string]string, len(header)) // normalised → first original

	for i, col := range header {
		trimmed := strings.TrimSpace(col)
		lower := strings.ToLower(trimmed)
		switch lower {
		case "email":
			emailIdx = i
		case "name":
			nameIdx = i
		default:
			if prev, exists := seenNorm[lower]; exists {
				warnings = append(warnings, fmt.Sprintf(
					"columns %q and %q both normalise to %q — only the last value will be used",
					prev, trimmed, lower,
				))
			} else {
				seenNorm[lower] = trimmed
				if lower != trimmed {
					warnings = append(warnings, fmt.Sprintf(
						"column %q normalised to lowercase %q — use {{.%s}} in templates",
						trimmed, lower, lower,
					))
				}
			}
			colMap[i] = lower
		}
	}

	if emailIdx == -1 {
		return nil, nil, fmt.Errorf("CSV file is missing a required 'email' column")
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

	return recipients, warnings, nil
}
