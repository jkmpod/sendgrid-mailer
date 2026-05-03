# Developing

> **Owner: Developer agent.** Edit this file when adding new how-to recipes,
> documenting a new pattern, or updating the skills index. Architectural
> decisions belong in `ARCHITECTURE.md`; review/test policy belongs in
> `AGENTS.md`.

This file is a working reference for adding code. It captures the conventions
already used in the project so new work blends in.

## Build / test / run

```bash
go mod tidy
go build ./...
go vet ./...
go test ./... -v
go run .                  # starts on PORT (default 8080)
```

`go run .` reads `.env` from the project root via `godotenv`. Copy
`.env.example` to `.env` first.

## After a PR is merged

Run `scripts/post-merge.sh` from the repo root once GitHub shows the PR as
merged. It checks that your working tree is clean, switches to `master`,
fast-forwards to the latest remote commit, attempts a safe delete of the
feature branch you were on, and prunes stale remote-tracking refs. The script
works in Git Bash on Windows and in standard Linux/macOS bash.

## Reusable skills

Each file under `.claude/skills/` is a deeper write-up of one pattern used in
this codebase. Read the relevant skill before adding code in that area.

| Skill | When you need it |
|-------|------------------|
| `error-handling.md` | Returning errors from any new function |
| `http-handler-closure.md` | A handler that needs config, mailer, or other deps |
| `json-request-response.md` | Any handler that reads or writes JSON |
| `httptest-handler-testing.md` | Writing tests for a handler |
| `mutex-shared-state.md` | Adding new package-level state shared across handlers |
| `sse-streaming.md` | Streaming progress to the browser |
| `table-driven-tests.md` | Default test style for any new function |
| `session-tracking.md` | End-of-session housekeeping |

## Recipe: add a new HTTP handler

1. Pick the file in `server/handlers/` that matches the feature, or create a
   new `<feature>.go` if no existing file fits.
2. Use a closure factory if the handler needs dependencies — see
   `http-handler-closure.md` and existing examples like `HandleSend(e, cfg)`.
3. Read JSON input with `json.NewDecoder(r.Body).Decode(&req)` and
   `defer r.Body.Close()`. Validate required fields and return 400 with a
   JSON body `{"error": "..."}` on failure — never `http.Error()` with a bare
   string.
4. Write JSON output with the project's `writeJSON` helper. Set the status
   code before encoding.
5. Register the route in `server/server.go` using Go 1.22+ method-aware
   routing: `mux.HandleFunc("POST /thing", handlers.HandleThing(...))`.
6. Write a table-driven test in `<feature>_test.go` using `httptest.NewRecorder`.

## Recipe: add a new piece of shared state

Most "remember the last X" state lives in package-level variables in
`server/handlers/` protected by a `sync.Mutex`. Follow the existing
`lastSubject` / `SetLastSubject` / `GetLastSubject` triplet:

```go
var (
    mu       sync.Mutex
    lastFoo  string
)

// SetLastFoo stores ... .
func SetLastFoo(s string) {
    mu.Lock()
    defer mu.Unlock()
    lastFoo = s
}

// GetLastFoo returns ... .
func GetLastFoo() string {
    mu.Lock()
    defer mu.Unlock()
    return lastFoo
}
```

If the state should be resettable for tests, add a `ResetXxx()` helper that
the test files can call in their setup.

## Recipe: add a runtime config override

The pattern is in `server/handlers/state.go` (or its sibling files):

1. Add a getter/setter pair for the override (mutex-protected).
2. Add an `EffectiveXxx(cfg *config.Config)` helper that returns the override
   if set, otherwise the config value.
3. Read it from `HandleConfig` (so the UI can display the effective value).
4. Accept it in the `POST /config` body and call the setter from
   `HandleConfigUpdate`.
5. Use `EffectiveXxx` everywhere in handlers — never read `cfg.Xxx` directly
   for an overridable field.

## Recipe: stream progress over SSE

`HandleSend` is the canonical example. Key points:

- Set `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `Connection: keep-alive`.
- After every write, call `flusher.Flush()` (assert `w.(http.Flusher)`).
- Format frames as `data: <json>\n\n`.
- The browser side uses `EventSource` in `templates/index.html`.

See `.claude/skills/sse-streaming.md` for the full template.

## Recipe: mock SendGrid in tests

Real SendGrid calls are forbidden in tests. Use `httptest.NewServer` to stand
up a fake endpoint and override the SendGrid base URL on the `Emailer`:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusAccepted)
}))
defer srv.Close()

e := mailer.NewEmailer(cfg)
e.SetBaseURL(srv.URL) // or however the test hooks the URL
```

Existing `mailer/sender_test.go` and the handler tests show the working
pattern.

## UI conventions (`templates/index.html`)

- Single file. No npm, no bundler, no external JS libraries — vanilla JS only.
- Layout uses CSS grid / flexbox; the page must be mobile-responsive.
- Every `<input>` has a `<label>`. Prefer `<main>`, `<nav>`, `<button>` over
  generic `<div>`s.
- Render server-supplied strings with text-content APIs, never via
  `template.HTML(...)` on the Go side. Flag any unescaped HTML insertion.
- Error states must be visible — never silently fail. Inline error banners,
  not `console.error` only.
- Test mode is read-only on the UI: a badge that reflects `GET /config`,
  never a toggle.

## Reference files when in doubt

- Routes and wiring → `server/server.go`
- JSON helpers and shared handler state → `server/handlers/state.go`,
  `server/handlers/confighandler.go`
- The simplest handler example → `server/handlers/compose.go`
- The SSE example → `server/handlers/send.go`
- Test patterns → any `*_test.go` in `server/handlers/`
