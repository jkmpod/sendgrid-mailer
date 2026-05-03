# Testing HTTP Handlers with httptest

## When to Use

Unit testing any `http.HandlerFunc` or `http.Handler` without starting a real server.

## Pattern

```go
func TestHandleFoo(t *testing.T) {
    tests := []struct {
        name       string
        body       string
        wantStatus int
        wantErr    string
    }{
        {
            name:       "valid request",
            body:       `{"name":"Alice"}`,
            wantStatus: http.StatusOK,
        },
        {
            name:       "missing name",
            body:       `{"name":""}`,
            wantStatus: http.StatusBadRequest,
            wantErr:    "name is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 1. Create a fake request.
            req := httptest.NewRequest("POST", "/foo", strings.NewReader(tt.body))
            req.Header.Set("Content-Type", "application/json")

            // 2. Create a response recorder.
            rr := httptest.NewRecorder()

            // 3. Call the handler directly.
            handler := HandleFoo(mockService)
            handler.ServeHTTP(rr, req)

            // 4. Assert status code.
            if rr.Code != tt.wantStatus {
                t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
            }

            // 5. Assert response body.
            if tt.wantErr != "" {
                var resp map[string]interface{}
                json.Unmarshal(rr.Body.Bytes(), &resp)
                if !strings.Contains(resp["error"].(string), tt.wantErr) {
                    t.Errorf("error = %q, want substring %q", resp["error"], tt.wantErr)
                }
            }
        })
    }
}
```

### Mocking External APIs

```go
// Create a mock server that returns a canned response.
sgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusAccepted)
}))
defer sgServer.Close()

// Inject sgServer.URL into the client under test.
```

### Temp File Helpers

```go
func writeTempCSV(t *testing.T, content string) string {
    t.Helper()
    dir := t.TempDir()
    path := filepath.Join(dir, "test.csv")
    os.WriteFile(path, []byte(content), 0644)
    return path
}
```

## Key Rules

- Use `strings.NewReader(jsonString)` for POST bodies.
- Set `Content-Type` header on the request when sending JSON.
- Use `rr.Code` for status, `rr.Body.Bytes()` for response body.
- Use `httptest.NewServer` to mock external APIs (e.g., SendGrid). Always `defer server.Close()`.
- Use `t.TempDir()` for test files — automatically cleaned up when the test ends.
- Combine with table-driven tests for comprehensive handler coverage.

## Example from This Codebase

`server/handlers/send_test.go` — `TestHandleSend` uses 5 table-driven cases for validation errors, plus `TestHandleSend_ValidRequest` and `TestHandleSend_PartialFailure` as standalone tests that create temp CSV files and exercise the full handler flow.

`mailer/sender_test.go` — `newTestEmailer` helper creates a mock SendGrid server and injects its URL into the Emailer's client for isolated API testing.
