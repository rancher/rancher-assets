#!/usr/bin/env bash
# Shared setup for bump-pr-builder scripts. Source this file: source "$(dirname "$0")/common.sh"

# Determine REPO_DIR (rancher-assets root) from this script's location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="${REPO_DIR:-$(cd "$SCRIPT_DIR/../../.." && pwd)}"

# Git remote name (may differ locally if using a fork)
REMOTE="${REMOTE:-origin}"

# Target branch for PR (defaults to current branch or main)
TARGET_BRANCH="${TARGET_BRANCH:-}"

# Skip git commits, push, and PR creation when true
DRY_RUN="${DRY_RUN:-false}"

# Source repo for PR body
SOURCE_REPO="${SOURCE_REPO:-rancher/rancher-assets}"

# Write to GitHub step summary if available, and always print to stdout
summary() {
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    echo "$@" >> "$GITHUB_STEP_SUMMARY"
  fi
  echo "$@"
}

# Write only to stdout (detailed logs, not in GH summary)
log() {
  echo "$@"
}

require_var() {
  local var="$1"
  if [ -z "${!var:-}" ]; then
    echo "ERROR: $var is required" >&2
    exit 1
  fi
}

require_repo_dir() {
  require_var REPO_DIR
  if [ ! -d "$REPO_DIR" ]; then
    echo "ERROR: REPO_DIR '$REPO_DIR' does not exist" >&2
    exit 1
  fi
}

# Get current branch or default to main
get_target_branch() {
  if [ -n "$TARGET_BRANCH" ]; then
    echo "$TARGET_BRANCH"
    return
  fi

  # Try to get current branch
  local current_branch
  current_branch=$(git -C "$REPO_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

  if [ -n "$current_branch" ] && [ "$current_branch" != "HEAD" ]; then
    echo "$current_branch"
  else
    echo "main"
  fi
}

# Commit all changes in REPO_DIR if any exist. Returns 1 if no changes, 0 on success.
commit_if_changed() {
  local message="$1"
  if git -C "$REPO_DIR" diff --quiet --exit-code && [ -z "$(git -C "$REPO_DIR" status --porcelain)" ]; then
    return 1
  fi

  if ! git -C "$REPO_DIR" add . 2>&1; then
    echo "ERROR: Failed to stage changes" >&2
    return 2
  fi

  if ! git -C "$REPO_DIR" commit -m "$message" 2>&1; then
    echo "ERROR: Failed to create commit" >&2
    return 2
  fi

  return 0
}
