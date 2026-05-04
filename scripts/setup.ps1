# setup.ps1 -- First-time setup for sendgrid-mailer (Windows / PowerShell)
# Run this once after cloning the repository.
#
# Usage:
#   cd sendgrid-mailer
#   .\scripts\setup.ps1

$ErrorActionPreference = "Stop"

function info  { Write-Host "  -> $args" -ForegroundColor Cyan }
function ok    { Write-Host "  OK $args" -ForegroundColor Green }
function error { Write-Host "ERROR: $args" -ForegroundColor Red; exit 1 }

Write-Host ""
Write-Host "sendgrid-mailer - first-time setup" -ForegroundColor White
Write-Host "===================================="
Write-Host ""

# -- 1. Check Go is installed --------------------------------------------------
info "Checking for Go..."
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host ""
    Write-Host "  Go is not installed on this machine." -ForegroundColor Yellow
    Write-Host "  Please download and install Go from:"
    Write-Host "    https://go.dev/dl/" -ForegroundColor Cyan
    Write-Host "  After installing, close and reopen PowerShell, then run this script again."
    exit 1
}
ok (go version)

# -- 2. Create .env from template ----------------------------------------------
info "Checking for .env file..."
if (-not (Test-Path ".env")) {
    Copy-Item ".env.example" ".env"
    ok ".env created from .env.example"
    Write-Host ""
    Write-Host "  NEXT: Open .env in any text editor (e.g. Notepad) and fill in:" -ForegroundColor Yellow
    Write-Host "    SENDGRID_API_KEY  - your SendGrid API key (starts with SG.)"
    Write-Host "    FROM_EMAIL        - the email address you want to send from"
    Write-Host "    FROM_NAME         - the name recipients will see as the sender"
    Write-Host ""
} else {
    ok ".env already exists - skipping copy"
}

# -- 3. Download Go dependencies -----------------------------------------------
info "Downloading dependencies..."
go mod download
if ($LASTEXITCODE -ne 0) { error "go mod download failed." }
ok "Dependencies downloaded"

# -- 4. Quick build check ------------------------------------------------------
info "Verifying the project builds..."
go build ./... 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { error "Build failed. Check that Go 1.23+ is installed." }
ok "Build succeeded"

Write-Host ""
Write-Host "Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Edit .env with your SendGrid API key and sender details"
Write-Host "  2. Start the app:  .\scripts\start.ps1"
Write-Host "  3. Open browser:   http://localhost:8080"
Write-Host ""
