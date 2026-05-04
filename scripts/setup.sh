#!/usr/bin/env bash
# setup.sh — First-time setup for sendgrid-mailer (macOS / Linux)
# Run this once after cloning the repository.
#
# Usage:
#   cd sendgrid-mailer
#   bash scripts/setup.sh

set -euo pipefail

info()  { printf '  -> %s\n' "$*"; }
ok()    { printf '  OK %s\n' "$*"; }
error() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

echo ""
echo "sendgrid-mailer — first-time setup"
echo "===================================="
echo ""

# ── 1. Check Go is installed ──────────────────────────────────────────────────
info "Checking for Go..."
if ! command -v go &>/dev/null; then
    echo ""
    echo "  Go is not installed on this machine."
    echo "  Please install it:"
    echo "    macOS:  brew install go   (or visit https://go.dev/dl/)"
    echo "    Linux:  https://go.dev/dl/"
    echo ""
    exit 1
fi
ok "$(go version)"

# ── 2. Create .env from template ──────────────────────────────────────────────
info "Checking for .env file..."
if [ ! -f .env ]; then
    cp .env.example .env
    ok ".env created from .env.example"
    echo ""
    echo "  NEXT: Open .env in any text editor and fill in:"
    echo "    SENDGRID_API_KEY  — your SendGrid API key (starts with SG.)"
    echo "    FROM_EMAIL        — the email address you want to send from"
    echo "    FROM_NAME         — the name recipients will see as the sender"
    echo ""
else
    ok ".env already exists — skipping copy"
fi

# ── 3. Download Go dependencies ───────────────────────────────────────────────
info "Downloading dependencies..."
go mod download
ok "Dependencies downloaded"

# ── 4. Quick build check ──────────────────────────────────────────────────────
info "Verifying the project builds..."
go build ./... >/dev/null 2>&1 || error "Build failed. Check that Go 1.23+ is installed."
ok "Build succeeded"

echo ""
echo "Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Edit .env with your SendGrid API key and sender details"
echo "  2. Start the app:  bash scripts/start.sh"
echo "  3. Open browser:   http://localhost:8080"
echo ""
