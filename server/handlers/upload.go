package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/loader"
	"github.com/jkmpod/sendgrid-mailer/models"
)

// HandleUpload returns an http.HandlerFunc that accepts a multipart/form-data
// POST with a CSV file field named "file". It saves the file to a temp
// directory, parses it with loader.LoadFromCSV, and returns JSON with the
// recipient count, column names, and a preview of the first 3 rows. The
// maximum upload size is taken from cfg.MaxUploadSizeMB.
func HandleUpload(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxBytes := int64(cfg.MaxUploadSizeMB) << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		defer r.Body.Close()

		if err := r.ParseMultipartForm(maxBytes); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "file too large or invalid multipart form",
			})
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "missing required 'file' field",
			})
			return
		}
		defer file.Close()

		// Save to a temp file so loader.LoadFromCSV can read it by path.
		tmpDir := os.TempDir()
		tmpPath := filepath.Join(tmpDir, "sendgrid-upload-"+header.Filename)
		dst, err := os.Create(tmpPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to create temp file",
			})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to save uploaded file",
			})
			return
		}
		// Close before reading so the file is flushed.
		dst.Close()

		recipients, err := loader.LoadFromCSV(tmpPath)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		columns := columnNames(recipients)
		preview := previewRows(recipients, 3)

		// Store column list for the compose endpoint.
		SetLastColumns(columns)
		SetLastFilePath(tmpPath)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"count":    len(recipients),
			"columns":  columns,
			"preview":  preview,
			"filePath": tmpPath,
		})
	}
}

// columnNames extracts the full set of column names from the first recipient.
func columnNames(recipients []models.EmailRecipient) []string {
	cols := []string{"email", "name"}
	if len(recipients) == 0 {
		return cols
	}
	for k := range recipients[0].CustomFields {
		cols = append(cols, k)
	}
	return cols
}

// previewRows returns up to n recipients as plain maps for JSON output.
func previewRows(recipients []models.EmailRecipient, n int) []map[string]string {
	if n > len(recipients) {
		n = len(recipients)
	}
	rows := make([]map[string]string, n)
	for i := 0; i < n; i++ {
		row := make(map[string]string, len(recipients[i].CustomFields)+2)
		row["email"] = recipients[i].Email
		row["name"] = recipients[i].Name
		for k, v := range recipients[i].CustomFields {
			row[k] = v
		}
		rows[i] = row
	}
	return rows
}

// writeJSON is a helper that sets the Content-Type header and writes a JSON
// response with the given status code. An encoding failure is logged — by the
// time we hit it, the status line is already on the wire so we can't recover.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[handlers] writeJSON encode failed: %v", err)
	}
}
