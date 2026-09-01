#!/usr/bin/env bash
# Runs 'make update' and checks if there are changes to commit.
# Returns:
#   0 - changes detected
#   2 - no changes (early exit - caller should skip PR creation)
#   1 - error occurred

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_repo_dir

summary ""
summary "## Running make update"

# Run make update
if ! (cd "$REPO_DIR" && make update); then
  summary "  ✗ make update failed"
  exit 1
fi

summary "  ✓ make update completed"

# Check if there are any changes using scripts/verify
# scripts/verify exits with 1 if there are changes (opposite of what we want)
summary ""
summary "## Checking for changes"

VERIFY_EXIT=0
(cd "$REPO_DIR" && ./scripts/verify) || VERIFY_EXIT=$?

if [ "$VERIFY_EXIT" -eq 0 ]; then
  # No changes detected
  summary "  ℹ️  No changes detected - repository is up to date"
  exit 2
fi

# Changes detected
summary "  ✓ Changes detected"
git -C "$REPO_DIR" status --porcelain dockerfiles/ lock.yaml | sed 's/^/    /'

exit 0
