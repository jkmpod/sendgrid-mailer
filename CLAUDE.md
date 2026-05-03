# sendgrid-mailer — agent context

> **Owner: Architect agent.** Keep this file under ~100 lines. It is the entry
> point that points at the other docs — it must not duplicate them.

## Identity

- **Language:** Go 1.23+ (pinned by `go.mod`).
- **Module:** `github.com/jkmpod/sendgrid-mailer`.
- **Purpose:** A self-hosted Go web app for sending bulk personalised email
  via the SendGrid v3 API, with a vanilla-JS browser UI.
- **External dependencies:** `sendgrid-go`, `rest`, `godotenv`. The full
  allowlist lives in `ARCHITECTURE.md`.

## Commands

```bash
go mod tidy
go build ./...
go vet ./...
go test ./... -v
go run .                 # serves on PORT (default 8080)
```

## Where to find things

| Topic | File |
|-------|------|
| User-facing intro, env vars, endpoints (canonical) | `README.md` |
| Package DAG, scope, dependency policy, runtime model | `ARCHITECTURE.md` |
| Review checklists, code style, test policy | `AGENTS.md` |
| Recipes for adding handlers, state, SSE, tests | `DEVELOPING.md` |
| Reusable pattern write-ups | `.claude/skills/` |
| Config template | `.env.example` |
| UI (single page, vanilla JS only) | `templates/index.html` |
| Archived session prompts (history only) | `docs/build-history.md` |

## Ground rules (the small, durable core)

1. **Standard library only**, except the documented allowlist in
   `ARCHITECTURE.md`. No external web frameworks, no external JS libraries.
2. **No auth, no database, no persistence** unless the user asks. Runtime
   state is in-memory and resets on restart — see `ARCHITECTURE.md`.
3. **Every exported symbol has a doc comment.**
4. **Table-driven tests** for new functions. SendGrid calls in tests must be
   mocked via `httptest.NewServer` — never real API calls.
5. **Don't change exported function signatures** without discussion.
6. **Simple, readable Go** over clever abstractions. The author is a Go
   learner — keep idioms standard.

## Doc ownership

Each doc has a single owner agent. A change touching multiple docs is a
coordination signal, not a problem to hide in one file.

| Doc | Owner |
|-----|-------|
| `CLAUDE.md`, `ARCHITECTURE.md` | Architect agent |
| `AGENTS.md` | QC agent |
| `DEVELOPING.md` | Developer agent |
| `README.md` | Architect (user-facing; canonical for endpoints/env vars) |

When in doubt about *which* doc a change belongs in: scope/policy/architecture
decisions go to the Architect; review or test checks go to QC; concrete code
recipes go to the Developer.
