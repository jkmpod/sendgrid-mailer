# AGENTS.md — Review Criteria for AI Code Reviewers

This file provides structured review checklists for AI tools (Jules, Claude, etc.)
that review pull requests on this repository.

## Project Context

- Language: Go 1.23+
- Module: github.com/jkmpod/sendgrid-mailer
- Architecture: config → models → loader → mailer → server/handlers → main.go
- UI: Single-page vanilla HTML/CSS/JS in templates/index.html
- External Context: Refer to [CLAUDE.md](./CLAUDE.md) for coding style preferences and project-specific details.

## Development Commands

- **Check:** `go mod tidy`
- **Build:** `go build -o main .`
- **Test:** `go test ./... -v`

## Standard Library Only

All Go code MUST use only the Go standard library unless the SendGrid SDK
is specifically required for API calls. Use `net/http` exclusively for routing
and HTTP handling — do NOT suggest or use external web frameworks (Gin, Echo, Chi).

Allowed external dependencies (exhaustive list):
- `github.com/sendgrid/sendgrid-go` — SendGrid API client
- `github.com/sendgrid/rest` — transitive dependency of sendgrid-go
- `github.com/joho/godotenv` — .env loading in main.go ONLY

Flag any PR that adds a new `require` line to go.mod.

## Go Handler Checklist

When reviewing HTTP handlers in `server/handlers/`, verify:

1. **Error responses are JSON** — every error path must return `{"error": "..."}` via writeJSON, never a bare string or `http.Error()`.
2. **Input validation** — all POST handlers validate required fields and return 400 with a descriptive error.
3. **No hardcoded values** — configuration from `config.Config` or runtime state, not literals.
4. **Thread safety** — shared mutable state protected by mutex with `defer mu.Unlock()`.
5. **No signature changes** — existing exported function signatures must not change without discussion.
6. **Table-driven tests** — every new handler/function needs table-driven tests using `net/http/httptest`.
7. **No real API calls in tests** — SendGrid calls must be mocked via `httptest.NewServer`.
8. **Resource management** — use `defer r.Body.Close()` when reading request bodies.

## Go Idioms & Standard Library

- **Error Handling:** Every function returning an `error` must be checked.
- **Method Validation:** Handlers must explicitly check `r.Method` or use Go 1.22+ method-aware routing (`"POST /path"`).
- **Logging:** Use `log.Printf` for warnings, not `fmt.Printf`.
- **Naming:** Struct fields: PascalCase; JSON tags: camelCase.

## Security & HTML

- **XSS Prevention:** Use `html/template` for rendering. Flag any use of `template.HTML()` (unescaped).
- **Forms:** Every `<input>` must have a `<label>`. Validate all form inputs for empty strings or invalid data.
- **Semantic HTML:** Prefer `<main>`, `<nav>`, and `<button>` over generic `<div>` tags.

## Code Style Rules

1. Simple, readable Go over clever abstractions.
2. Every exported symbol has a doc comment.
3. No authentication, databases, or persistence unless explicitly requested.

## Package Dependency Order

Changes must respect the import DAG — reject any circular dependency:

```
config → models → loader → mailer → server/handlers → main.go
```
