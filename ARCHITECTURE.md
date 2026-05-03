# Architecture

> **Owner: Architect agent.** Edit this file when changing scope, the package
> graph, the dependency policy, the runtime model, or the roadmap. Code-style
> and review rules belong in `AGENTS.md`; how-to recipes belong in
> `DEVELOPING.md`.

## Toolchain

- **Go 1.23+** (pinned by `go.mod` directive `go 1.23.4`).
- Module: `github.com/jkmpod/sendgrid-mailer`.

## Package dependency graph

Imports flow strictly left-to-right. Any circular dependency is a defect.

```
config → models → loader → mailer → server/handlers → server → main
```

- `config` depends on the standard library only.
- `models` is a leaf data package (no methods, no imports beyond stdlib).
- `loader` depends on `models`.
- `mailer` depends on `config`, `models`, and the SendGrid SDK.
- `server/handlers` depends on `config`, `mailer`, `loader`, `models`.
- `server` wires routes; `main` wires `config` → `server`.

## Scope boundaries

The application is intentionally small. Do not add the following without an
explicit request from the user:

- **Authentication / authorization.** No login, no session cookies, no API tokens
  for the UI. The app is designed to be reachable only by its operator.
- **Database / persistence.** All runtime state is in-memory and resets on
  restart. This includes `lastSubject`, the send log, runtime config overrides,
  and the last uploaded CSV columns/path.
- **External web framework.** Routing is `net/http` only — no Gin, Echo, Chi.
- **Frontend framework.** The UI is a single `templates/index.html` of vanilla
  HTML / CSS / JS. No npm, no bundler, no React/Vue/etc.

## External dependency allowlist

Every external dependency must appear here. Adding a new `require` line to
`go.mod` requires updating this list and is grounds for review rejection
otherwise.

| Module | Purpose |
|--------|---------|
| `github.com/sendgrid/sendgrid-go` | SendGrid v3 API client and mail helpers |
| `github.com/sendgrid/rest` | Transitive dependency of `sendgrid-go` |
| `github.com/joho/godotenv` | `.env` loader, used in `main.go` only |

## Runtime model

- **Configuration** is loaded once at startup from environment variables (see
  `.env.example`) into `config.Config`. Most values can be overridden at
  runtime via `POST /config`; overrides are stored in mutex-protected
  package-level variables in `server/handlers` and lost on restart.
- **State sharing across handlers** uses package-level variables protected by
  `sync.Mutex`. Each shared variable has a getter and setter that lock and
  unlock around access. Examples: `lastSubject`, `lastColumns`, `lastFilePath`,
  send log entries, runtime config overrides.
- **Long-running sends** stream progress to the browser over Server-Sent
  Events (`text/event-stream`). The connection is held open by `HandleSend`
  for the duration of the bulk send.
- **SendGrid API calls** go through the `mailer.Emailer`. In tests, the SendGrid
  base URL is overridden to point at an `httptest.NewServer` — no real network
  calls.
- **Test mode** is a config-time toggle (`TEST_MODE=true` + `TEST_EMAILS=...`)
  that diverts every send to a fixed list of test addresses with `[TEST] `
  prefixed to the subject. Controlled by env var only — there is no UI toggle.

## Roadmap and intentional non-features

- A `campaign/` package was discussed in early sessions as a placeholder. It
  is not implemented and there is no current plan to add it. Remove this entry
  if and when the decision is made.
- Persistence (any flavour) is deliberately out of scope. If the user later
  asks for it, design discussion goes here before code is written.
