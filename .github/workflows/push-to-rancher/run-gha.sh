#!/usr/bin/env bash
# GHA entry point: orchestrates the full rancher/rancher update workflow.
# Called from push-to-rancher.yml after token generation and rancher checkout.
#
# Required env vars (set by push-to-rancher.yml):
#   TAG          - rancher-assets tag (e.g. v2.16-20260901T1200Z)
#   GH_TOKEN     - GitHub app token with access to rancher/rancher
#   APP_USER     - GitHub app slug for commit attribution (e.g. "my-app[bot]")
#   SOURCE_REPO  - source repo (github.repository)
#   ASSETS_DIR   - path to rancher-assets workspace ($GITHUB_WORKSPACE)
#   RANCHER_DIR  - path where rancher/rancher was cloned (must exist before script runs)
#
# Optional env vars:
#   RANCHER_BRANCHES_OVERRIDE - space or comma-separated list of branches to process

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_var TAG
require_var GH_TOKEN
require_var APP_USER
require_var SOURCE_REPO
require_var ASSETS_DIR
require_var RANCHER_DIR # Expected to be cloned before this script is run

export ASSETS_DIR RANCHER_DIR DRY_RUN RANCHER_BRANCHES_OVERRIDE

# Extract minor version and filter branches
MINOR_VERSION=$(get_tag_minor_version "$TAG")
FILTERED_BRANCHES=$(filter_branches_for_tag "$TAG")

summary "## Push to rancher/rancher"
summary "- Tag: \`$TAG\`"
summary "- Version: \`v$MINOR_VERSION\`"
summary "- Target branches: \`$FILTERED_BRANCHES\`"
summary ""

# Validate image exists before proceeding
if ! validate_image_exists "$TAG"; then
  summary "## ERROR: Image validation failed"
  summary "Cannot proceed - image must be published first"
  exit 1
fi

# Configure git identity for commits (GHA only - fresh clone deleted at workflow end)
log "Fetching user ID for $APP_USER..."
user_id=$(gh api "/users/$APP_USER" --jq .id 2>&1) || {
  summary "## ERROR: Failed to fetch user ID"
  summary '```'
  summary "$user_id"
  summary '```'
  exit 1
}
log "  ✓ User ID: $user_id"

git -C "$RANCHER_DIR" config user.name "$APP_USER"
git -C "$RANCHER_DIR" config user.email "${user_id}+${APP_USER}@users.noreply.github.com"

summary ""
summary "## Creating PRs"

export SOURCE_REPO="${SOURCE_REPO:-rancher/rancher-assets}"
bash "$SCRIPT_DIR/create-prs.sh"

summary ""
summary "## Workflow Complete"
