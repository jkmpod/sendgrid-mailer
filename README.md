# sendgrid-mailer

A self-hosted Go web app for sending bulk email via the SendGrid API. Upload a CSV of recipients, compose an HTML template with per-recipient personalisation, and send — with real-time progress streamed to the browser.

## Features

- **CSV upload** — drag-and-drop a `.csv` file; the app detects columns automatically and shows a preview
- **Template editor** — compose HTML with `{{.ColumnName}}` placeholders; click column chips to insert tags at the cursor
- **Bulk sending** — recipients are batched automatically with configurable batch size and rate delay
- **Real-time progress** — batch results stream to the browser via Server-Sent Events (SSE)
- **Partial failure handling** — if some batches fail, the rest still send; per-batch errors are reported
- **Test mode** — send to a configured list of test addresses instead of real recipients (controlled by env var, not the UI)
- **Activity logs** — view recent SendGrid delivery events filtered by subject, with a 10-minute cooldown after sending

## Requirements

- Go 1.23+
- A [SendGrid API key](https://docs.sendgrid.com/ui/account-and-settings/api-keys) with Mail Send permission

## Quick Start

```bash
git clone https://github.com/jkmpod/sendgrid-mailer.git
cd sendgrid-mailer

# Create a .env file (or export the variables directly)
cp .env.example .env   # then edit with your values

go run .
```

Open `http://localhost:8080` in your browser.

## Configuration

All configuration is via environment variables. A `.env` file in the project root is loaded automatically on startup.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SENDGRID_API_KEY` | Yes | — | SendGrid API key (starts with `SG.`) |
| `FROM_EMAIL` | Yes | — | Sender email address |
| `FROM_NAME` | Yes | — | Sender display name |
| `MAX_BATCH_SIZE` | No | `1000` | Max recipients per SendGrid API call |
| `RATE_DELAY_MS` | No | `100` | Milliseconds to wait between batches |
| `PORT` | No | `8080` | HTTP server listen port |
| `TEST_MODE` | No | `false` | When `true`, emails go only to `TEST_EMAILS` |
| `TEST_EMAILS` | When `TEST_MODE=true` | — | Comma-separated list of test recipient addresses |

## Project Structure

```
sendgrid-mailer/
  main.go                      # Entry point — loads .env, starts server
  config/
    config.go                  # Environment variable loading and validation
  models/
    recipient.go               # EmailRecipient data struct
  loader/
    csv.go                     # CSV parsing into EmailRecipient slices
  mailer/
    emailer.go                 # Emailer struct and constructor
    batch.go                   # ChunkRecipients helper
    personalize.go             # BuildMail — template rendering + SendGrid payload
    sender.go                  # SendBatch, SendBulk, SendTest
  server/
    server.go                  # HTTP server and route registration
    handlers/
      upload.go                # POST /upload — CSV file upload
      send.go                  # POST /send — bulk/test email sending (SSE)
      logs.go                  # GET /logs — SendGrid Activity Feed proxy
      compose.go               # GET /compose — column metadata for editor
      confighandler.go         # GET /config — test mode status + last subject
      state.go                 # Shared handler state (lastSubject)
  templates/
    index.html                 # Single-page UI (HTML, CSS, vanilla JS)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Serves the web UI |
| `POST` | `/upload` | Accepts multipart CSV upload (max 10 MB). Returns recipient count, column names, and a 3-row preview. |
| `POST` | `/send` | Accepts JSON `{"subject", "template", "filePath"}`. Streams batch progress via SSE, or returns JSON in test mode. |
| `GET` | `/logs` | Proxies the SendGrid Activity Feed API. Accepts optional `?subject=` query param. |
| `GET` | `/compose` | Returns column names and file path from the last CSV upload. |
| `GET` | `/config` | Returns `{"testMode": bool, "lastSubject": string}`. |

## Running Tests

```bash
go test ./... -v
```

Tests use `net/http/httptest` to mock SendGrid API responses — no real API calls are made.

## Dependencies

| Module | Purpose |
|--------|---------|
| [sendgrid-go](https://github.com/sendgrid/sendgrid-go) | SendGrid v3 API client and mail helpers |
| [godotenv](https://github.com/joho/godotenv) | Loads `.env` file into environment variables |

All other code uses the Go standard library.

## License

MIT
