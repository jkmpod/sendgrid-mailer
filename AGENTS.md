# AGENTS.md — Review Criteria for AI Code Reviewers

> **Owner: QC agent.** Edit this file when adding/removing review checks,
> tightening style rules, or changing test expectations. Architecture and
> dependency policy live in `ARCHITECTURE.md`; how-to recipes live in
> `DEVELOPING.md`.

This file provides structured review checklists for AI tools (Jules, Claude,
etc.) that review pull requests on this repository.

## Project context

- Language: **Go 1.23+** (pinned by `go.mod`).
- Module: `github.com/jkmpod/sendgrid-mailer`.
- Architecture and package DAG: see [`ARCHITECTURE.md`](./ARCHITECTURE.md).
- UI: single-page vanilla HTML/CSS/JS in `templates/index.html`.
- Coding style and project preferences: see [`CLAUDE.md`](./CLAUDE.md) and
  [`DEVELOPING.md`](./DEVELOPING.md).

## Development commands

- **Tidy:** `go mod tidy`
- **Build:** `go build -o main .`
- **Vet:** `go vet ./...`
- **Test:** `go test ./... -v`

## Standard library only

All Go code MUST use only the Go standard library unless the SendGrid SDK is
specifically required for API calls. Use `net/http` exclusively for routing
and HTTP handling — do NOT suggest or use external web frameworks (Gin, Echo,
Chi).

The exhaustive allowlist of permitted external dependencies lives in
[`ARCHITECTURE.md`](./ARCHITECTURE.md#external-dependency-allowlist). Flag any
PR that adds a new `require` line to `go.mod` without first updating that
allowlist.

## Go handler checklist

When reviewing HTTP handlers in `server/handlers/`, verify:

1. **Error responses are JSON** — every error path must return
   `{"error": "..."}` via `writeJSON`, never a bare string or `http.Error()`.
2. **Input validation** — all POST handlers validate required fields and
   return 400 with a descriptive error.
3. **No hardcoded values** — configuration from `config.Config` or runtime
   state, not literals.
4. **Thread safety** — shared mutable state protected by mutex with
   `defer mu.Unlock()`.
5. **No signature changes** — existing exported function signatures must not
   change without discussion.
6. **Table-driven tests** — every new handler/function needs table-driven
   tests using `net/http/httptest`.
7. **No real API calls in tests** — SendGrid calls must be mocked via
   `httptest.NewServer`.
8. **Resource management** — use `defer r.Body.Close()` when reading request
   bodies.

## Go idioms and standard library

- **Error handling:** Every function returning an `error` must be checked.
- **Method validation:** Handlers must explicitly check `r.Method` or use
  Go 1.22+ method-aware routing (`"POST /path"`).
- **Logging:** Use `log.Printf` for warnings, not `fmt.Printf`.
- **Naming:** Struct fields: PascalCase; JSON tags: camelCase.

## Security and HTML

- **XSS prevention:** Use `html/template` for rendering. Flag any use of
  `template.HTML()` (unescaped).
- **Forms:** Every `<input>` must have a `<label>`. Validate all form inputs
  for empty strings or invalid data.
- **Semantic HTML:** Prefer `<main>`, `<nav>`, and `<button>` over generic
  `<div>` tags.

## Code style rules

1. Simple, readable Go over clever abstractions.
2. Every exported symbol has a doc comment.
3. No authentication, databases, or persistence unless explicitly requested
   (see [`ARCHITECTURE.md`](./ARCHITECTURE.md#scope-boundaries)).

## Package dependency order

Changes must respect the import DAG defined in
[`ARCHITECTURE.md`](./ARCHITECTURE.md#package-dependency-graph). Reject any
PR that introduces a circular dependency.
