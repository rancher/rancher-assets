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

# Check if there are any changes in generated files
summary ""
summary "## Checking for changes"

# Refresh git index to avoid false positives
git -C "$REPO_DIR" update-index --refresh >/dev/null 2>&1 || true

# Check for changes in generated files (dockerfiles/ and lock.yaml)
if [ -z "$(git -C "$REPO_DIR" status --porcelain dockerfiles/ lock.yaml)" ]; then
  # No changes detected
  summary "  ℹ️  No changes detected - repository is up to date"
  exit 2
fi

# Changes detected
summary "  ✓ Changes detected in generated files:"
git -C "$REPO_DIR" status --porcelain dockerfiles/ lock.yaml | sed 's/^/    /'

exit 0
