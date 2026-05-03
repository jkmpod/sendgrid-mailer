# HTTP Handler Closure (Dependency Injection)

## When to Use

Any HTTP handler that needs access to configuration, services, or other dependencies. This replaces global variables.

## Pattern

```go
// The outer function takes dependencies and returns an http.HandlerFunc.
// It runs ONCE at server startup.
func HandleFoo(svc *SomeService, cfg *config.Config) http.HandlerFunc {
    // The inner function runs PER REQUEST.
    return func(w http.ResponseWriter, r *http.Request) {
        // svc and cfg are captured by the closure — available on every request.
        result, err := svc.DoWork(r.Context())
        if err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{
                "error": err.Error(),
            })
            return
        }
        writeJSON(w, http.StatusOK, result)
    }
}

// Registration in server setup:
mux.HandleFunc("POST /foo", HandleFoo(svc, cfg))
```

## Key Rules

- The outer function signature captures dependencies; the inner closure handles the request. This separates "what do I need" from "what do I do per request".
- Use `http.HandlerFunc` as the return type (not `http.Handler`) unless you need middleware chaining.
- Register with Go 1.22+ method-aware routing: `"GET /path"`, `"POST /path"`.
- For handlers with no dependencies, use a plain function: `func HandleHealth(w http.ResponseWriter, r *http.Request)`.
- Never use global variables for dependencies — always pass them through the closure.

## Example from This Codebase

`server/handlers/send.go` — `HandleSend(e *mailer.Emailer, cfg *config.Config) http.HandlerFunc` captures the emailer and config. The closure checks `cfg.TestMode` to decide between `SendTest` and `SendBulk`.

`server/handlers/logs.go` — `HandleLogs(apiKey string) http.HandlerFunc` captures just the API key string. The closure uses it to set the Authorization header on outbound requests.

`server/server.go` — Registration: `srv.mux.HandleFunc("POST /send", handlers.HandleSend(e, cfg))`.
