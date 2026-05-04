# start.ps1 — Start sendgrid-mailer (Windows / PowerShell)
#
# Usage:
#   cd sendgrid-mailer
#   .\scripts\start.ps1

$ErrorActionPreference = "Stop"

if (-not (Test-Path ".env")) {
    Write-Host "ERROR: .env file not found." -ForegroundColor Red
    Write-Host "       Run .\scripts\setup.ps1 first to create it."
    exit 1
}

# Read PORT from .env for the browser hint (default 8080 if not set)
$port = "8080"
Get-Content ".env" | Where-Object { $_ -match "^\s*PORT\s*=" } | ForEach-Object {
    $port = ($_ -split "=", 2)[1].Trim()
}

Write-Host ""
Write-Host "Starting sendgrid-mailer..." -ForegroundColor Cyan
Write-Host "Open your browser at: http://localhost:$port" -ForegroundColor Green
Write-Host "Press Ctrl+C to stop the server."
Write-Host ""

go run .
