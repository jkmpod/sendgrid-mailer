#!/usr/bin/env bash
# check-gofmt.sh — fails if any Go file in the module needs "gofmt -s" formatting.
# Used by the pre-commit quality gate; see DEVELOPING.md for details.
#
# gofmt -s -l exits 0 even when it lists unformatted files, so a naive hook
# won't block commits. This wrapper captures the diff output and exits 1 if
# it is non-empty.
set -euo pipefail

diff_output=$(gofmt -s -d .)
if [ -n "$diff_output" ]; then
    printf 'gofmt -s found unformatted files. Run: gofmt -s -w .\n\n%s\n' "$diff_output" >&2
    exit 1
fi
