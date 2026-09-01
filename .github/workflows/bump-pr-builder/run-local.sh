#!/usr/bin/env bash
# Local entry point for testing bump-pr-builder workflow.
#
# Usage:
#   ./run-local.sh [--dry-run] [--remote origin] [--target-branch main]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Defaults
DRY_RUN="false"
REMOTE="origin"
TARGET_BRANCH=""

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    --remote)
      REMOTE="$2"
      shift 2
      ;;
    --target-branch)
      TARGET_BRANCH="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: $0 [--dry-run] [--remote REMOTE] [--target-branch BRANCH]" >&2
      exit 1
      ;;
  esac
done

# Set up environment
export REPO_DIR="${REPO_DIR:-$(cd "$SCRIPT_DIR/../../.." && pwd)}"
export REMOTE DRY_RUN TARGET_BRANCH
export GH_TOKEN="${GH_TOKEN:-$(gh auth token)}"
export SOURCE_REPO="${SOURCE_REPO:-rancher/rancher-assets}"

# Source common for helper functions
source "$SCRIPT_DIR/common.sh"

echo "## Bump PR Builder (local)"
echo "- Repository dir: $REPO_DIR"
echo "- Remote: $REMOTE"
echo "- Dry run: $DRY_RUN"
echo "- Target branch: ${TARGET_BRANCH:-<current>}"
echo ""

# Run make update and check for changes
UPDATE_EXIT=0
bash "$SCRIPT_DIR/run-update.sh" || UPDATE_EXIT=$?

if [ "$UPDATE_EXIT" -eq 2 ]; then
  # No changes detected - early exit
  echo ""
  echo "## Workflow Complete"
  echo "No changes detected - repository is up to date"
  exit 0
elif [ "$UPDATE_EXIT" -ne 0 ]; then
  # Update failed
  echo ""
  echo "## Workflow Failed"
  echo "make update encountered an error"
  exit 1
fi

# Changes detected - create PR
bash "$SCRIPT_DIR/create-pr.sh"

echo ""
echo "## Workflow Complete"

if [ "$DRY_RUN" = "true" ]; then
  echo ""
  echo "NOTE: This was a dry run. Changes were committed locally but not pushed."
  echo "Review the commits in $REPO_DIR and run without --dry-run to push and create PR."
fi
