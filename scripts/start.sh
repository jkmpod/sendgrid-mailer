#!/usr/bin/env bash
# start.sh — Start sendgrid-mailer (macOS / Linux)
#
# Usage:
#   cd sendgrid-mailer
#   bash scripts/start.sh

set -euo pipefail

if [ ! -f .env ]; then
    echo "ERROR: .env file not found."
    echo "       Run 'bash scripts/setup.sh' first to create it."
    exit 1
fi

# Read PORT from .env for the browser hint (default 8080 if not set)
port=$(grep -E '^\s*PORT\s*=' .env | cut -d= -f2 | tr -d ' ' || true)
port=${port:-8080}

echo ""
echo "Starting sendgrid-mailer..."
echo "Open your browser at: http://localhost:${port}"
echo "Press Ctrl+C to stop the server."
echo ""

go run .
