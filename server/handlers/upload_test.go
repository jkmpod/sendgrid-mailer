package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// createMultipartBody builds a multipart form with a single CSV file field.
func createMultipartBody(t *testing.T, fieldName, fileName, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte(content))
	writer.Close()
	return body, writer.FormDataContentType()
}

func TestHandleUpload(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		fileName   string
		csv        string
		wantStatus int
		wantErr    string // substring in error response; "" means success
		wantCount  int    // expected "count" in success response
	}{
		{
			name:       "valid CSV upload",
			fieldName:  "file",
			fileName:   "test.csv",
			csv:        "email,name,company\nalice@ex.com,Alice,Acme\nbob@ex.com,Bob,Globex\n",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "missing file field",
			fieldName:  "wrong_field",
			fileName:   "test.csv",
			csv:        "email,name\na@b.com,A\n",
			wantStatus: http.StatusBadRequest,
			wantErr:    "missing required 'file' field",
		},
		{
			name:       "malformed CSV (missing email column)",
			fieldName:  "file",
			fileName:   "bad.csv",
			csv:        "name,company\nAlice,Acme\n",
			wantStatus: http.StatusBadRequest,
			wantErr:    "missing a required 'email' column",
		},
		{
			name:       "empty CSV file",
			fieldName:  "file",
			fileName:   "empty.csv",
			csv:        "",
			wantStatus: http.StatusBadRequest,
			wantErr:    "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := createMultipartBody(t, tt.fieldName, tt.fileName, tt.csv)

			req := httptest.NewRequest("POST", "/upload", body)
			req.Header.Set("Content-Type", contentType)

			rr := httptest.NewRecorder()
			HandleUpload(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response JSON: %v", err)
			}

			if tt.wantErr != "" {
				errMsg, ok := resp["error"].(string)
				if !ok {
					t.Fatalf("expected error field in response, got: %v", resp)
				}
				if !strings.Contains(errMsg, tt.wantErr) {
					t.Errorf("error = %q, want substring %q", errMsg, tt.wantErr)
				}
				return
			}

			count, ok := resp["count"].(float64)
			if !ok {
				t.Fatalf("expected count field in response, got: %v", resp)
			}
			if int(count) != tt.wantCount {
				t.Errorf("count = %d, want %d", int(count), tt.wantCount)
			}

			// Verify preview is present and has at most 3 rows.
			preview, ok := resp["preview"].([]interface{})
			if !ok {
				t.Fatalf("expected preview field in response, got: %v", resp)
			}
			if len(preview) > 3 {
				t.Errorf("preview has %d rows, want at most 3", len(preview))
			}

			// Verify columns is present.
			cols, ok := resp["columns"].([]interface{})
			if !ok {
				t.Fatalf("expected columns field in response, got: %v", resp)
			}
			if len(cols) < 2 {
				t.Errorf("expected at least 2 columns (email, name), got %d", len(cols))
			}

			// Verify filePath is returned.
			fp, ok := resp["filePath"].(string)
			if !ok || fp == "" {
				t.Error("expected non-empty filePath in response")
			}
		})
	}
}

func TestHandleUpload_OversizedFile(t *testing.T) {
	// Create a CSV larger than maxUploadSize (10 MB).
	bigContent := "email,name\n" + strings.Repeat("user@example.com,User Name With Extra Padding To Fill Space\n", 200000)

	body, contentType := createMultipartBody(t, "file", "big.csv", bigContent)

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)

	rr := httptest.NewRecorder()
	HandleUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for oversized file", rr.Code, http.StatusBadRequest)
	}
}
