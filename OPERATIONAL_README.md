# sendgrid-mailer — Operations Guide

This guide is for team members who will **run and use** the app day-to-day. No coding knowledge is needed.

---

## What does this app do?

sendgrid-mailer is a tool you run on your own computer. It opens a website in your browser where you can:

1. Upload a spreadsheet (CSV) of email recipients
2. Write an email template with personalised fields (e.g. a recipient's name or company)
3. Send the emails to everyone on the list via SendGrid — with live progress shown on screen
4. Review delivery logs pulled straight from SendGrid

---

## Before you begin

You need two things installed on your computer:

### 1. Go (the runtime the app is built on)

- Download from: **https://go.dev/dl/**
- Choose the installer for your operating system (Windows `.msi`, macOS `.pkg`, or Linux `.tar.gz`)
- Run the installer and follow the prompts — the defaults are fine
- When done, open a new terminal and type `go version`; you should see something like `go version go1.23.4`

### 2. Git (to download this app)

- Download from: **https://git-scm.com/downloads**
- Run the installer with default options

You also need a **SendGrid account** with an API key. If you don't have one, ask your technical contact — they can create an API key at `https://app.sendgrid.com/settings/api_keys` with *Mail Send* permission.

---

## First-time setup

Open a terminal (PowerShell on Windows, Terminal on macOS/Linux) and run these commands one at a time:

```bash
# Download the app
git clone https://github.com/jkmpod/sendgrid-mailer.git
cd sendgrid-mailer
```

**Windows (PowerShell):**
```powershell
.\scripts\setup.ps1
```

**macOS / Linux:**
```bash
bash scripts/setup.sh
```

The script will:
- Check that Go is installed
- Create your personal settings file (`.env`) from the template
- Download required dependencies
- Verify the app builds correctly

---

## Configuring the app

After setup, open the `.env` file in any text editor (Notepad, TextEdit, etc.). It lives in the `sendgrid-mailer` folder.

Fill in the three required settings:

```
SENDGRID_API_KEY=SG.xxxxxxxxxxxxxxxxxxxxxxxx.xxxxx...
FROM_EMAIL=yourname@yourcompany.com
FROM_NAME=Your Name or Team Name
```

| Setting | What to put here |
|---------|-----------------|
| `SENDGRID_API_KEY` | The API key from your SendGrid account. It always starts with `SG.` |
| `FROM_EMAIL` | The email address the messages will appear to come from. Must be a **verified sender** in SendGrid. |
| `FROM_NAME` | The display name recipients will see in their inbox (e.g. `"Acme Marketing"`) |

### Optional settings

These have sensible defaults — only change them if you need to:

| Setting | Default | What it does |
|---------|---------|--------------|
| `PORT` | `8080` | The browser address port. Change to `9090` if 8080 is in use on your machine. |
| `MAX_BATCH_SIZE` | `1000` | How many recipients are sent per API call. Leave at 1000 unless SendGrid advises otherwise. |
| `RATE_DELAY_MS` | `100` | Milliseconds to pause between batches. Increase to `500` if you see rate-limit errors. |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum CSV file size in megabytes. |

### Test mode (strongly recommended for first runs)

By default the app starts in **test mode** — emails go only to a safe list of addresses you control, not to real recipients. This protects against accidental sends.

```
TEST_MODE=true
TEST_EMAILS=you@yourcompany.com,colleague@yourcompany.com
```

When you are ready to send to real recipients, change `TEST_MODE` to `false`.

> **Important:** always run a test-mode send first to verify your template renders correctly before switching to `TEST_MODE=false`.

---

## Starting the app

Every time you want to use the app, open a terminal, navigate to the `sendgrid-mailer` folder, and run:

**Windows:**
```powershell
.\scripts\start.ps1
```

**macOS / Linux:**
```bash
bash scripts/start.sh
```

Then open your browser and go to: **http://localhost:8080**

Leave the terminal window open while you use the app. To stop the app, click into the terminal and press `Ctrl + C`.

---

## Preparing your recipient list (CSV format)

The app accepts a `.csv` file (a spreadsheet saved as CSV). The only required column is `email`. A `name` column is strongly recommended for personalisation.

### Minimum format

```
email,name
alice@example.com,Alice
bob@example.com,Bob
```

### Extended format (custom fields for personalisation)

You can add any extra columns — they become available as template placeholders:

```
email,name,company,role
alice@example.com,Alice,Acme,Manager
bob@example.com,Bob,Globex,Director
```

### Rules

- The first row must be column headings
- The `email` column is required (lowercase)
- Column names must not contain spaces (use `first_name` not `first name`)
- Save the file as CSV (not `.xlsx`) — in Excel: *File → Save As → CSV UTF-8*
- Maximum file size: 10 MB (adjustable via `MAX_UPLOAD_SIZE_MB` in `.env`)

### Downloading a blank template

Save this as `recipients.csv` and fill it in:

```csv
email,name,company
recipient@example.com,Full Name,Company Name
```

---

## Using the app — step by step

### Step 1: Upload your CSV

1. Open the app in your browser at **http://localhost:8080**
2. Click **Choose File** (or drag and drop your CSV onto the upload area)
3. The app will show you:
   - The number of recipients detected
   - A preview of the first few rows
   - The column names it found

If you see an error, check that your CSV has an `email` column and is saved in CSV format.

### Step 2: Compose your email

1. Enter the **Subject** line
2. In the **Template** editor, write your email in HTML
3. Use `{{.ColumnName}}` to insert personalised values — for example:
   - `{{.name}}` — inserts the recipient's name
   - `{{.company}}` — inserts their company (if that column exists in your CSV)
4. Click the **column chip buttons** to insert a placeholder at the cursor position without typing it manually
5. Use the **Preview** button to see what the email will look like for the first recipient

**Example template:**

```html
<p>Hi {{.name}},</p>

<p>We wanted to reach out to everyone at <strong>{{.company}}</strong>
about our upcoming webinar.</p>

<p>Best regards,<br>The Team</p>
```

### Step 3: (Optional) Add categories

In the **Categories** field, enter comma-separated labels for this send. Categories appear in your SendGrid Activity Feed and stats, making it easier to filter and report on different campaigns:

```
newsletter, march-2026
```

Rules: maximum 10 categories, each no longer than 255 characters.

### Step 4: Send

1. Click **Send** (or **Send Test** if `TEST_MODE=true`)
2. Watch the live progress bar — each batch result appears as it completes
3. Green results mean success; red means that batch failed (the rest will still send)
4. A summary appears when the send completes

---

## Checking delivery logs

Click the **Logs** tab (or navigate to **http://localhost:8080** and look for the logs section) to view recent SendGrid delivery events.

- You can filter by subject line to find a specific campaign
- Logs are pulled live from the SendGrid Activity Feed — they reflect SendGrid's own delivery records
- There is a 10-minute cooldown after sending before logs refresh, to allow SendGrid time to process events

> Logs are read-only. To export or analyse delivery data in detail, log in to your SendGrid dashboard at **https://app.sendgrid.com**.

---

## Common issues

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| App won't start — `SENDGRID_API_KEY environment variable is required` | `.env` file missing or API key not filled in | Open `.env` and add your API key |
| App won't start — `cannot find 'go'` | Go is not installed or not on the PATH | Install Go from https://go.dev/dl/ and reopen your terminal |
| CSV upload fails — `missing a required 'email' column` | CSV doesn't have a column named exactly `email` | Rename the column to `email` (lowercase) in your spreadsheet and re-export |
| CSV upload fails — `file too large` | File exceeds the upload limit | Increase `MAX_UPLOAD_SIZE_MB` in `.env`, or split the CSV into smaller files |
| Emails sent but not arriving | `TEST_MODE=true` is still set | Change `TEST_MODE=false` in `.env` and restart the app |
| Rate limit errors during send | Sending too fast | Increase `RATE_DELAY_MS` to `500` or `1000` in `.env` |
| Port already in use | Another program is using port 8080 | Change `PORT=9090` in `.env` and open http://localhost:9090 |
| Template renders `<no value>` | Column name in `{{.ColumnName}}` doesn't match CSV | Check spelling and capitalisation — `{{.Name}}` and `{{.name}}` are different |

---

## Stopping the app

Click into the terminal window where the app is running and press **Ctrl + C**. The app stops immediately — no data is lost.

---

## Quick-reference card

```
First-time setup
  Windows:   .\scripts\setup.ps1
  Mac/Linux: bash scripts/setup.sh

Start the app
  Windows:   .\scripts\start.ps1
  Mac/Linux: bash scripts/start.sh
  Browser:   http://localhost:8080

Stop the app
  Press Ctrl+C in the terminal

Edit settings
  Open .env in any text editor
  Restart the app after saving changes
```
