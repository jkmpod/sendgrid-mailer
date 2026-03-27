package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/loader"
	"github.com/jkmpod/sendgrid-mailer/mailer"
)

// sendRequest is the expected JSON body for the /send endpoint.
type sendRequest struct {
	Subject  string `json:"subject"`
	Template string `json:"template"`
	FilePath string `json:"filePath"`
}

// HandleSend returns an http.HandlerFunc that accepts a JSON POST, loads
// recipients from a CSV, and sends email in batches. Progress is streamed to
// the client using Server-Sent Events (text/event-stream) so the log panel
// updates in real time. When cfg.TestMode is true, emails are sent only to
// cfg.TestEmails using the first CSV row for personalisation.
func HandleSend(e *mailer.Emailer, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req sendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
			return
		}

		if req.FilePath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "filePath is required",
			})
			return
		}
		if req.Subject == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "subject is required",
			})
			return
		}
		if req.Template == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "template is required",
			})
			return
		}

		log.Printf("[send] request: subject=%q file=%q testMode=%v", req.Subject, req.FilePath, cfg.TestMode)

		recipients, err := loader.LoadFromCSV(req.FilePath)
		if err != nil {
			log.Printf("[send] CSV load failed: %v", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("failed to load CSV: %v", err),
			})
			return
		}
		log.Printf("[send] loaded %d recipients from CSV", len(recipients))

		// Test mode: send only to test emails using the first CSV row.
		if cfg.TestMode {
			if len(recipients) == 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "CSV has no recipients to use as test data",
				})
				return
			}
			result, err := e.SendTest(cfg.TestEmails, req.Subject, req.Template, recipients[0])
			if err != nil {
				log.Printf("[send] SendTest error: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": err.Error(),
				})
				return
			}
			log.Printf("[send] test complete: sent=%d failed=%d", result.TotalSent, result.TotalFailed)
			SetLastSubject(req.Subject)
			AppendSendLog(SendLogEntry{
				Time:        time.Now(),
				Subject:     "[TEST] " + req.Subject,
				TotalSent:   result.TotalSent,
				TotalFailed: result.TotalFailed,
				TestMode:    true,
			})
			resp := sendResultToJSON(result)
			resp["testMode"] = true
			writeJSON(w, http.StatusOK, resp)
			return
		}

		// Set up Server-Sent Events streaming.
		flusher, ok := w.(http.Flusher)
		if !ok {
			// Fallback: if flushing isn't supported, do a normal JSON response.
			result, err := e.SendBulk(recipients, req.Subject, req.Template)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": err.Error(),
				})
				return
			}
			if result.TotalSent > 0 {
				SetLastSubject(req.Subject)
			}
			AppendSendLog(SendLogEntry{
				Time:        time.Now(),
				Subject:     req.Subject,
				TotalSent:   result.TotalSent,
				TotalFailed: result.TotalFailed,
				TestMode:    false,
			})
			resp := sendResultToJSON(result)
			resp["testMode"] = false
			writeJSON(w, http.StatusOK, resp)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		chunks := mailer.ChunkRecipients(recipients, e.MaxBatchSize)
		var totalSent, totalFailed int
		type batchErrorJSON struct {
			BatchIndex int    `json:"batchIndex"`
			Error      string `json:"error"`
		}
		var batchErrors []batchErrorJSON

		for i, chunk := range chunks {
			if i > 0 {
				time.Sleep(time.Duration(e.RateDelayMS) * time.Millisecond)
			}

			_, err := e.SendBatch(chunk, req.Subject, req.Template)
			if err != nil {
				totalFailed += len(chunk)
				batchErrors = append(batchErrors, batchErrorJSON{
					BatchIndex: i,
					Error:      err.Error(),
				})
				sseEvent(w, "batch", map[string]interface{}{
					"batch":  i + 1,
					"total":  len(chunks),
					"status": "failed",
					"error":  err.Error(),
				})
			} else {
				totalSent += len(chunk)
				sseEvent(w, "batch", map[string]interface{}{
					"batch":  i + 1,
					"total":  len(chunks),
					"status": "ok",
					"sent":   len(chunk),
				})
			}
			flusher.Flush()
		}

		log.Printf("[send] complete: totalSent=%d totalFailed=%d", totalSent, totalFailed)

		if totalSent > 0 {
			SetLastSubject(req.Subject)
		}
		AppendSendLog(SendLogEntry{
			Time:        time.Now(),
			Subject:     req.Subject,
			TotalSent:   totalSent,
			TotalFailed: totalFailed,
			TestMode:    false,
		})

		// Send the final summary event.
		sseEvent(w, "done", map[string]interface{}{
			"totalSent":   totalSent,
			"totalFailed": totalFailed,
			"batchErrors": batchErrors,
			"testMode":    false,
		})
		flusher.Flush()
	}
}

// sendResultToJSON converts a mailer.SendResult to a JSON-friendly map.
func sendResultToJSON(sr mailer.SendResult) map[string]interface{} {
	errors := make([]map[string]interface{}, len(sr.BatchErrors))
	for i, be := range sr.BatchErrors {
		errors[i] = map[string]interface{}{
			"batchIndex": be.BatchIndex,
			"error":      be.Err.Error(),
		}
	}
	return map[string]interface{}{
		"totalSent":   sr.TotalSent,
		"totalFailed": sr.TotalFailed,
		"batchErrors": errors,
	}
}

// sseEvent writes a single Server-Sent Event to the response.
func sseEvent(w http.ResponseWriter, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
}
