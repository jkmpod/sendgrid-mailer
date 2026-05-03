#!/usr/bin/env bash
# post-merge.sh — run this after a PR is merged on GitHub to bring your
# local working copy back to a clean master state.
#
# Usage:
#   bash scripts/post-merge.sh
#
# What it does:
#   1. Aborts if the working tree is dirty (uncommitted changes).
#   2. If already on master: fast-forward pull + prune and exit.
#   3. Otherwise: switch to master, fast-forward pull, safely delete the
#      previous feature branch, then prune remote-tracking refs.
#
# Exit codes:
#   0  — success
#   1  — dirty working tree or pull failure

set -euo pipefail

DEFAULT_BRANCH="master"

# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------

info()  { printf '  -> %s\n' "$*"; }
error() { printf 'ERROR: %s\n' "$*" >&2; }

# ---------------------------------------------------------------------------
# 1. Refuse to run on a dirty tree
# ---------------------------------------------------------------------------

info "Checking working tree..."
if ! git diff-index --quiet HEAD --; then
    error "Working tree is dirty. Commit or stash your changes before running this script."
    exit 1
fi
info "Working tree is clean."

# ---------------------------------------------------------------------------
# 2. Determine the current branch
# ---------------------------------------------------------------------------

current_branch="$(git rev-parse --abbrev-ref HEAD)"
info "Current branch: ${current_branch}"

# ---------------------------------------------------------------------------
# 3. If already on master, just pull and prune
# ---------------------------------------------------------------------------

if [ "${current_branch}" = "${DEFAULT_BRANCH}" ]; then
    info "Already on ${DEFAULT_BRANCH}. Pulling latest..."
    git pull --ff-only origin "${DEFAULT_BRANCH}"
    info "Pruning stale remote-tracking refs..."
    git fetch --prune
    info "Done. HEAD is now:"
    git log -1 --oneline
    exit 0
fi

# ---------------------------------------------------------------------------
# 4. Switch to master and fast-forward pull
# ---------------------------------------------------------------------------

info "Switching to ${DEFAULT_BRANCH}..."
git checkout "${DEFAULT_BRANCH}"

info "Fast-forward pulling origin/${DEFAULT_BRANCH}..."
git pull --ff-only origin "${DEFAULT_BRANCH}"

# ---------------------------------------------------------------------------
# 5. Safely delete the previous feature branch
# ---------------------------------------------------------------------------

info "Attempting to delete branch '${current_branch}' (safe delete)..."
if git branch -d "${current_branch}"; then
    info "Branch '${current_branch}' deleted."
else
    printf '\n'
    printf 'NOTE: git branch -d refused to delete '"'"'%s'"'"'.\n' "${current_branch}"
    printf 'This usually means the branch has commits not yet merged into %s.\n' "${DEFAULT_BRANCH}"
    printf 'To inspect the difference:\n'
    printf '  git log %s..%s\n' "${DEFAULT_BRANCH}" "${current_branch}"
    printf 'To force-delete once you are sure:\n'
    printf '  git branch -D %s\n' "${current_branch}"
    printf '\n'
fi

# ---------------------------------------------------------------------------
# 6. Prune stale remote-tracking refs
# ---------------------------------------------------------------------------

info "Pruning stale remote-tracking refs..."
git fetch --prune

# ---------------------------------------------------------------------------
# 7. Report final state
# ---------------------------------------------------------------------------

info "Done. HEAD is now:"
git log -1 --oneline
