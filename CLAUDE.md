[go doc -all output for config, mailer, server/handlers]

package config // import "github.com/jkmpod/sendgrid-mailer/config"


TYPES

type Config struct {
        APIKey       string
        FromEmail    string
        FromName     string
        MaxBatchSize int
        RateDelayMS  int
}
    Config holds all application configuration read from environment variables.

func Load() (*Config, error)
    Load reads configuration from environment variables and returns a populated
    Config pointer. It returns an error if any required variable is missing or
    if an integer variable contains a non-numeric value.

package mailer // import "github.com/jkmpod/sendgrid-mailer/mailer"


FUNCTIONS

func BuildMail(
        from *mail.Email,
        subject string,
        htmlTemplate string,
        recipients []models.EmailRecipient,
) (*mail.SGMailV3, error)
    BuildMail constructs an SGMailV3 message with one Personalization per
    recipient. The htmlTemplate string is parsed as a Go text/template and
    executed once for each recipient. Template data includes "Email", "Name",
    and every key from recipient.CustomFields.

func ChunkRecipients(recipients []models.EmailRecipient, batchSize int) [][]models.EmailRecipient
    ChunkRecipients splits a slice of recipients into batches of at most
    batchSize elements. It returns a slice of slices rather than a channel
    because the full list is already in memory, the caller needs random access
    to report per-batch errors, and a simple slice is easier to test and reason
    about than a channel.


TYPES

type BatchError struct {
        BatchIndex int
        Err        error
}
    BatchError records a failure for a specific batch during bulk sending.

type Emailer struct {
        MaxBatchSize int
        RateDelayMS  int

        // Has unexported fields.
}
    Emailer holds configuration and the SendGrid client needed to send emails.

func NewEmailer(cfg *config.Config) *Emailer
    NewEmailer creates an Emailer from application config. It initialises the
    SendGrid client using the API key from cfg.

func (e *Emailer) SendBatch(
        recipients []models.EmailRecipient,
        subject string,
        htmlTemplate string,
) (map[string]interface{}, error)
    SendBatch sends a single batch of recipients. It builds the mail message,
    calls the SendGrid API, and returns the parsed response body.

func (e *Emailer) SendBulk(
        recipients []models.EmailRecipient,
        subject string,
        htmlTemplate string,
) (SendResult, error)
    SendBulk splits recipients into batches, sends each one, and collects
    results. It does NOT stop on the first batch error — partial success
    is a valid and expected outcome. A top-level error is returned only if
    something systemic fails (e.g. the template is unparseable). A time.Sleep of
    RateDelayMS milliseconds is inserted between batches.

type SendResult struct {
        TotalSent   int
        TotalFailed int
        BatchErrors []BatchError
}
    SendResult summarises the outcome of a bulk send operation. Partial success
    is expected — check BatchErrors for per-batch details.

package handlers // import "github.com/jkmpod/sendgrid-mailer/server/handlers"


FUNCTIONS

func HandleCompose(w http.ResponseWriter, r *http.Request)
    HandleCompose returns the column names and file path from the most
    recent CSV upload. This is a helper endpoint for the template editor — no
    persistence is needed.

func HandleLogs(apiKey string) http.HandlerFunc
    HandleLogs returns an http.HandlerFunc that calls the SendGrid Activity Feed
    API to fetch the last 50 message events and returns the raw JSON response to
    the client.

func HandleSend(e *mailer.Emailer) http.HandlerFunc
    HandleSend returns an http.HandlerFunc that accepts a JSON POST, loads
    recipients from a CSV, and sends email in batches. Progress is streamed to
    the client using Server-Sent Events (text/event-stream) so the log panel
    updates in real time.

func HandleUpload(w http.ResponseWriter, r *http.Request)
    HandleUpload accepts a multipart/form-data POST with a CSV file field
    named "file". It saves the file to a temp directory, parses it with
    loader.LoadFromCSV, and returns JSON with the recipient count, column names,
    and a preview of the first 3 rows.

func SetLastColumns(cols []string)
    SetLastColumns stores the column names from the most recent CSV upload.

func SetLastFilePath(path string)
    SetLastFilePath stores the file path from the most recent CSV upload.

[Ground Rules]
Ground rules for this session:
1. Implement one package at a time — do not jump ahead to the next package
2. Prefer simple, readable Go over clever abstractions — I am a Go learner
3. After each file, explain every exported symbol and every error return point
4. Use only the Go standard library unless the SendGrid SDK is specifically required
5. Do not add authentication, databases, or persistence unless I ask
6. Do not implement the campaign/ package — create a placeholder file only
7. Generate table-driven tests for every function before moving on
8. Ask me to verify tests pass before continuing to the next file

[Session 1 prompt]
You are helping me build a Go web app called sendgrid-mailer.
Module name: github.com/jkmpod/sendgrid-mailer
Go version: 1.22+

Today's task: implement the three foundation packages only.
Do not touch mailer/, server/, or main.go in this session.

--- config/config.go ---
Create a Config struct with these exported fields:
  APIKey       string
  FromEmail    string
  FromName     string
  MaxBatchSize int
  RateDelayMS  int

Create a Load() function that reads each field from environment variables:
  SENDGRID_API_KEY → APIKey
  FROM_EMAIL       → FromEmail
  FROM_NAME        → FromName
  MAX_BATCH_SIZE   → MaxBatchSize  (default 1000 if not set)
  RATE_DELAY_MS    → RateDelayMS   (default 100 if not set)

Load() must return (*Config, error). If APIKey, FromEmail, or FromName
are empty, return a descriptive error. Do not use any third-party
config library — use os.Getenv and strconv only.

--- models/recipient.go ---
Create an EmailRecipient struct with these exported fields:
  Email        string
  Name         string
  CustomFields map[string]string

No methods needed on this struct. It is a plain data carrier.

--- loader/csv.go ---
Create LoadFromCSV(filePath string) ([]models.EmailRecipient, error).

Rules:
- First row of CSV is the header. Column names map to CustomFields keys.
- "email" column (case-insensitive) maps to EmailRecipient.Email
- "name" column (case-insensitive) maps to EmailRecipient.Name
- All other columns go into CustomFields map
- Skip rows where the email field is empty, log a warning
- Return an error if the file cannot be opened or the CSV is malformed
- Use encoding/csv from the standard library only

--- After each file ---
Explain to me:
1. Every exported symbol and why it is exported
2. Every error return point and what triggers it
3. Any Go idiom used that differs from Python

--- Tests ---
After implementing all three files, generate table-driven tests for:
- config: valid env vars, missing required vars, invalid integer vars
- loader: valid CSV, missing email column, empty rows, malformed CSV

[Session 2 prompt]
Continuing sendgrid-mailer. Sessions 1 foundation packages are complete.
The following packages exist and compile cleanly:
  config, models, loader

Today's task: implement the mailer/ package across four files.

--- mailer/emailer.go ---
Define the Emailer struct. Exported fields:
  MaxBatchSize int
  RateDelayMS  int
Unexported fields:
  apiKey     string
  fromEmail  string
  fromName   string
  client     *sendgrid.Client

Create NewEmailer(cfg *config.Config) *Emailer.
It should initialise the SendGrid client using cfg.APIKey.

--- mailer/batch.go ---
Create ChunkRecipients(
  recipients []models.EmailRecipient,
  batchSize int,
) [][]models.EmailRecipient

This is a pure function with no errors — it always produces valid chunks.
Write a brief comment explaining why it returns a slice of slices, not a channel.

--- mailer/personalize.go ---
Create BuildMail(
  subject string,
  htmlTemplate string,
  recipients []models.EmailRecipient,
) (*mail.SGMailV3, error)

Rules:
- Use Go's text/template package to execute htmlTemplate per recipient
- Template data is a map built from recipient.CustomFields merged with:
    "Email" → recipient.Email
    "Name"  → recipient.Name
- If template parsing or execution fails for any recipient, return nil and the error
- Use github.com/sendgrid/sendgrid-go/helpers/mail for SGMailV3

--- mailer/sender.go ---
Define these two types:
  BatchError struct { BatchIndex int; Err error }
  SendResult struct { TotalSent int; TotalFailed int; BatchErrors []BatchError }

Create SendBatch(
  recipients []models.EmailRecipient,
  subject string,
  htmlTemplate string,
) (map[string]interface{}, error)

Create SendBulk(
  recipients []models.EmailRecipient,
  subject string,
  htmlTemplate string,
) (SendResult, error)

CRITICAL rule for SendBulk:
  Do NOT stop on the first batch error.
  Collect each BatchError into SendResult.BatchErrors and continue.
  Only return a top-level error if something systemic fails (e.g. template is unparseable).
  Partial success is a valid and expected outcome.
  Add a time.Sleep(RateDelayMS) between batches.

--- After each file ---
Explain every error return point and what the caller should do with it.

--- Tests ---
Write table-driven tests for:
- ChunkRecipients: empty input, single batch, multiple batches, exact boundary
- SendBulk: all succeed, one batch fails, all fail

Use mock HTTP responses for SendGrid calls — do not make real API calls in tests.

[Session 3 prompt]
Continuing sendgrid-mailer. Packages config, models, loader, mailer are complete.

Today's task: implement server/ and server/handlers/.

--- server/server.go ---
Create a Server struct with fields:
  mailer    *mailer.Emailer
  config    *config.Config

Create NewServer(cfg *config.Config) *Server.
Register these routes using net/http only (no external router):
  GET  /           → serve templates/index.html
  POST /upload     → handlers.HandleUpload
  POST /send       → handlers.HandleSend
  GET  /logs       → handlers.HandleLogs

--- server/handlers/upload.go ---
HandleUpload accepts a multipart/form-data POST with a CSV file field named "file".
It should:
- Save the file temporarily to os.TempDir()
- Call loader.LoadFromCSV() on it
- Return JSON: { "count": N, "columns": ["col1","col2",...], "preview": [first 3 rows] }
- Return a 400 error with a JSON error message if the file is missing or malformed

--- server/handlers/send.go ---
HandleSend accepts a JSON POST body:
  { "subject": "...", "template": "...", "filePath": "..." }

It should:
- Call loader.LoadFromCSV(filePath)
- Call mailer.SendBulk()
- Return JSON matching the SendResult struct:
  { "totalSent": N, "totalFailed": N, "batchErrors": [...] }
- Stream progress to the client using Server-Sent Events (text/event-stream)
  so the log panel updates in real time

--- server/handlers/logs.go ---
HandleLogs calls the SendGrid Activity Feed API to fetch the last 50 message events.
Endpoint: GET https://api.sendgrid.com/v3/messages?limit=50
Return the raw JSON response to the client.
Use net/http for the outbound call — no SDK wrapper needed here.

--- server/handlers/compose.go ---
HandleCompose is a helper endpoint (GET /compose) that returns the current
template and column names for the editor. No persistence needed — return
the last uploaded column list from a package-level variable for now.

--- After implementation ---
List every endpoint, its method, expected request shape, and response shape.
Do not add authentication in this session — that is a future concern.

--- Tests ---
Write handler tests using net/http/httptest for:
- Upload: valid CSV, missing file, oversized file
- Send: valid request, missing filePath, SendBulk returns partial failure

[Session 4 prompt]
Continuing sendgrid-mailer. All Go packages and HTTP endpoints are complete and tested.

Today's task: implement the browser UI in templates/index.html.
Use plain HTML, CSS, and vanilla JavaScript only — no frameworks or build tools.

The UI must have four sections on one page:

1. CSV UPLOADER
   - File input that accepts .csv only
   - On selection, POST to /upload and display:
     - Number of recipients loaded
     - Column names detected (shown as chips/tags)
     - Preview table of first 3 rows

2. HTML EDITOR
   - A textarea for composing the email HTML
   - A text input for the subject line
   - Display the detected column names as clickable chips above the editor
   - Clicking a chip inserts {{.ColName}} at the cursor position in the textarea
   - A "Preview" button that renders the template with the first CSV row as sample data

3. SEND CONTROLS
   - A "Send Bulk Email" button that POSTs to /send
   - Disable the button while sending is in progress
   - Show a progress indicator during sending

4. LOG PANEL
   - Updates in real time via Server-Sent Events from /send
   - Shows each batch result as it completes: batch number, sent count, errors
   - A "View SendGrid Activity" button that fetches /logs and renders a table:
     Columns: timestamp, to_email, status, subject

Constraints:
- No npm, no bundler, no external JS libraries
- Mobile-responsive layout using CSS grid or flexbox
- Use semantic HTML elements
- Error states must be visible — never silently fail in the UI

[Session 5 prompt]
Continuing sendgrid-mailer. All Go packages and HTTP endpoints are complete and tested.

Today's task: implement the browser UI in templates/index.html.
Use plain HTML, CSS, and vanilla JavaScript only — no frameworks or build tools.

The UI must have four sections on one page:

1. CSV UPLOADER
   - File input that accepts .csv only
   - On selection, POST to /upload and display:
     - Number of recipients loaded
     - Column names detected (shown as chips/tags)
     - Preview table of first 3 rows

2. HTML EDITOR
   - A textarea for composing the email HTML
   - A text input for the subject line
   - Display the detected column names as clickable chips above the editor
   - Clicking a chip inserts {{.ColName}} at the cursor position in the textarea
   - A "Preview" button that renders the template with the first CSV row as sample data

3. SEND CONTROLS
   - A "Send Bulk Email" button that POSTs to /send
   - Disable the button while sending is in progress
   - Show a progress indicator during sending

4. LOG PANEL
   - Updates in real time via Server-Sent Events from /send
   - Shows each batch result as it completes: batch number, sent count, errors
   - A "View SendGrid Activity" button that fetches /logs and renders a table:
     Columns: timestamp, to_email, status, subject

Constraints:
- No npm, no bundler, no external JS libraries
- Mobile-responsive layout using CSS grid or flexbox
- Use semantic HTML elements
- Error states must be visible — never silently fail in the UI

[Session 6 prompt]
A test mode feature needs to be added to the existing sendgrid-mailer Go app.
All five sessions are complete. Do not modify any existing package interfaces.
Only modify the specific files listed below.

--- FEATURE DESCRIPTION ---
When test mode is active:
  - Emails are sent ONLY to a configured list of test email addresses
  - The subject line is prefixed with [TEST]
  - The email body is personalised using the FIRST row of the uploaded CSV
    (as if the test recipients were that person)
  - The actual recipient list is ignored entirely
  - The log panel must clearly indicate TEST MODE was active for that send

--- config/config.go ---
Add these fields to the existing Config struct:
  TestMode   bool
  TestEmails []string

Load them from environment variables:
  TEST_MODE=true/false        → TestMode   (default false)
  TEST_EMAILS=a@x.com,b@x.com → TestEmails (comma-separated, split with strings.Split)

If TestMode is true but TestEmails is empty, return a descriptive error from Load().

--- mailer/sender.go ---
Add a new method on Emailer:

  func (e *Emailer) SendTest(
      testEmails []string,
      subject string,
      htmlTemplate string,
      firstRecipient models.EmailRecipient,
  ) (SendResult, error)

Rules:
  - Build one EmailRecipient per test email address, copying all fields
    from firstRecipient but replacing Email with the test address
  - Prefix subject with "[TEST] " before passing to BuildMail
  - Send as a single batch (no chunking needed)
  - Return a SendResult using the same struct as SendBulk

Do NOT modify SendBulk or SendBatch.

--- server/handlers/send.go ---
Modify HandleSend to check config.TestMode:
  - If true: call mailer.SendTest() using cfg.TestEmails and the first
    recipient from the loaded CSV
  - If false: call mailer.SendBulk() as before
  - Add a "testMode": true/false field to the JSON response so the UI
    can display the correct state in the log panel

--- templates/index.html ---
In the send controls section, add a read-only indicator (not a toggle —
test mode is controlled by env var, not the UI):
  - Show a visible badge: "TEST MODE ACTIVE" in amber when TestMode is true
  - The badge should be hidden when TestMode is false
  - The server must expose a GET /config endpoint returning
    {"testMode": true/false} so the UI can read this on page load

--- server/handlers/ (new file) ---
Create server/handlers/confighandler.go:
  HandleConfig returns JSON: {"testMode": bool}
  Register GET /config in server/server.go

--- TESTS ---
Add table-driven tests for SendTest in mailer/sender_test.go:
  - test emails receive personalised content from first CSV row
  - subject is correctly prefixed with [TEST]
  - empty testEmails returns an error
  - SendBulk is NOT called (verify test recipients only)

Add a test for HandleConfig in server/handlers/:
  - returns correct testMode value from config

--- CONSTRAINTS ---
  - Do not add a UI toggle for test mode — env var is the only control
  - Do not modify models/, loader/, or mailer/batch.go
  - Do not change any existing function signatures
  - Prefer simple, readable Go — no new abstractions needed
  - After each file, explain every change made and why

[Session 7 prompt]
Add three connected behaviours to the log panel.
Only modify the files listed below — do not touch mailer/, models/, loader/, or config/.

--- CONTEXT ---
The /config endpoint already exists in server/handlers/confighandler.go.
HandleSend already exists in server/handlers/send.go.
The UI already has a log panel and a "View SendGrid Activity" button.

--- NEW FILE: server/handlers/state.go ---
Create this file to hold shared handler state:

  package handlers

  import "sync"

  var (
      mu          sync.Mutex
      lastSubject string
  )

  func SetLastSubject(s string) {
      mu.Lock()
      defer mu.Unlock()
      lastSubject = s
  }

  func GetLastSubject() string {
      mu.Lock()
      defer mu.Unlock()
      return lastSubject
  }

Add a comment explaining why a mutex is needed here even for 
a single string variable.

--- server/handlers/send.go ---
After a successful SendBulk or SendTest call, store the subject:
  SetLastSubject(subject)

Extract subject from the incoming JSON request body before calling
send — it is already present in the request, just needs to be saved.
Do not change the function signature or response shape.

--- server/handlers/confighandler.go ---
Add lastSubject to the JSON response:

  Current:  {"testMode": bool}
  Updated:  {"testMode": bool, "lastSubject": string}

Call GetLastSubject() to populate the field.
If no send has happened yet, lastSubject will be an empty string — 
that is correct behaviour, not an error.

--- templates/index.html ---
Implement the full log panel state machine:

1. COOLDOWN TIMER (after send completes)
   - Record the send completion time in a JS variable: lastSendTime
   - Disable the "View Logs" button and subject input for 10 minutes
   - Show a countdown label: "Logs available in 8:42" updating every second
   - Use setInterval for the countdown — clear it when timer expires
   - When timer expires: enable the button and input, hide the countdown

2. PAGE LOAD BEHAVIOUR
   - On DOMContentLoaded, fetch GET /config
   - Store response.lastSubject in a JS variable: lastSentSubject
   - If lastSubject is non-empty, pre-populate the subject input field
   - If lastSubject is empty, leave the field blank and disable 
     the View Logs button until a subject is entered

3. SUBJECT VALIDATION (when View Logs is clicked)
   - Read the current value of the subject input field
   - Compare it against lastSentSubject (case-insensitive trim)
   - If they match: fetch /logs?subject=<value> directly
   - If they do not match: show an inline warning banner:
       "This subject differs from your last send ([lastSentSubject]).
        Fetching anyway."
     Then fetch /logs?subject=<value> after a 1.5 second delay
     so the user has time to read the warning
   - If subject field is empty: show inline error "Enter a subject 
     to search" and do not fetch

4. LOG TABLE
   - Display results with columns: time sent, recipient, status, subject
   - Show "No results found for this subject" if the array is empty
   - Show "SendGrid logs unavailable" if the fetch itself fails

--- TESTS ---

In server/handlers/ add or extend tests:

  TestSetGetLastSubject:
    - SetLastSubject then GetLastSubject returns same value
    - Default value before any set is empty string
    - Concurrent reads and writes do not panic (use t.Parallel)

  TestHandleConfig (extend existing):
    - Returns empty lastSubject before any send
    - Returns correct lastSubject after SetLastSubject is called

  TestHandleSend (extend existing):
    - After a successful send, GetLastSubject() returns the 
      subject from that request
    - After a failed send, GetLastSubject() is not updated

--- CONSTRAINTS ---
  - Do not use any JS framework or library — vanilla JS only
  - Do not add a database or file persistence for lastSubject —
    in-memory is correct for now
  - The cooldown timer must be purely client-side — the server 
    has no awareness of it
  - Do not change any function signatures in mailer/ or loader/
  - After all changes: go build ./... must be silent
  - go test ./... -v must all pass
  - Explain the mutex in state.go in plain language as a comment