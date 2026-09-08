#!/usr/bin/env bash
# GHA entry point: orchestrates the chart reference update workflow.
# Called from pr-builder-cron.yml after token generation.
#
# Required env vars (set by pr-builder-cron.yml):
#   GH_TOKEN     - GitHub app token with access to rancher/rancher-assets
#   APP_USER     - GitHub app slug for commit attribution (e.g. "my-app[bot]")
#   SOURCE_REPO  - source repo (github.repository)
#   REPO_DIR     - path to rancher-assets workspace ($GITHUB_WORKSPACE)
#
# Optional env vars:
#   TARGET_BRANCH - target branch for PR (default: current branch)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_var GH_TOKEN
require_var APP_USER
require_var SOURCE_REPO
require_var REPO_DIR

export REPO_DIR DRY_RUN TARGET_BRANCH SOURCE_REPO

summary "## Bump PR Builder"
summary "- Repository: \`$SOURCE_REPO\`"
summary "- Target branch: \`${TARGET_BRANCH:-<current>}\`"
summary ""

# Configure git identity for commits (GHA only - CI environment)
log "Fetching user ID for $APP_USER..."
user_id=$(gh api "/users/$APP_USER" --jq .id 2>&1) || {
  summary "## ERROR: Failed to fetch user ID"
  summary '```'
  summary "$user_id"
  summary '```'
  exit 1
}
log "  ✓ User ID: $user_id"

git -C "$REPO_DIR" config user.name "$APP_USER"
git -C "$REPO_DIR" config user.email "${user_id}+${APP_USER}@users.noreply.github.com"

# Run make update and check for changes
UPDATE_EXIT=0
bash "$SCRIPT_DIR/run-update.sh" || UPDATE_EXIT=$?

if [ "$UPDATE_EXIT" -eq 2 ]; then
  # No changes detected - early exit (success)
  summary ""
  summary "## Workflow Complete"
  summary "No changes detected - repository is up to date"
  exit 0
elif [ "$UPDATE_EXIT" -ne 0 ]; then
  # Update failed
  summary ""
  summary "## Workflow Failed"
  summary "make update encountered an error"
  exit 1
fi

# Changes detected - create PR
bash "$SCRIPT_DIR/create-pr.sh"

summary ""
summary "## Workflow Complete"
