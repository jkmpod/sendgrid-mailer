# JSON Request/Response in Handlers

## When to Use

Any handler that receives JSON input and/or returns JSON output.

## Pattern

```go
// 1. Define an unexported request struct with json tags.
type fooRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func HandleFoo(svc *Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 2. Decode the JSON body.
        var req fooRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]string{
                "error": "invalid JSON body",
            })
            return
        }

        // 3. Validate required fields — one check per field, early return.
        if req.Name == "" {
            writeJSON(w, http.StatusBadRequest, map[string]string{
                "error": "name is required",
            })
            return
        }

        // 4. Do work, then respond.
        result := svc.Process(req.Name)
        writeJSON(w, http.StatusOK, map[string]interface{}{
            "result": result,
            "count":  42,
        })
    }
}

// 5. Shared helper — define once, use in all handlers.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}
```

## Key Rules

- Use an **unexported** struct (`fooRequest`, not `FooRequest`) for request types scoped to one handler.
- Always check `Decode` error first — catches malformed JSON, wrong types, etc.
- Use `map[string]string{"error": "..."}` for error responses (consistent shape).
- Use `map[string]interface{}` for responses with mixed types (strings, ints, bools, nested objects).
- The `writeJSON` helper sets Content-Type, writes the status code, and encodes in one call. Define it once in the handlers package.
- Never call `w.WriteHeader()` twice — the first call wins and subsequent calls log a warning.

## Example from This Codebase

`server/handlers/send.go` — `sendRequest` struct (unexported) with Subject, Template, FilePath fields. Decode + 4 validation checks (invalid JSON, empty filePath, empty subject, empty template), each returning 400 with a specific error message.

`server/handlers/upload.go` — `writeJSON` helper defined at line 116, reused by all handlers in the package.
