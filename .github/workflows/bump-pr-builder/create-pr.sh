#!/usr/bin/env bash
# Creates a PR with the updated chart references and generated files.
#
# Required env vars:
#   GH_TOKEN     - GitHub token for PR creation
#   REPO_DIR     - Path to rancher-assets clone
#   SOURCE_REPO  - Source repo for PR body (e.g. rancher/rancher-assets)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_var GH_TOKEN
require_repo_dir

# Determine target branch
TARGET_BRANCH=$(get_target_branch)
summary ""
summary "## Creating PR against \`$TARGET_BRANCH\`"

# Create feature branch with timestamp
BRANCH_NAME="bot/bump-chart-refs-$(date +%Y%m%d-%H%M%S)"
log "  - Creating branch \`$BRANCH_NAME\`"

if ! git -C "$REPO_DIR" checkout -b "$BRANCH_NAME" 2>&1; then
  summary "  ✗ Failed to create branch \`$BRANCH_NAME\`"
  exit 1
fi

# Commit changes
COMMIT_MSG="Bump chart references and update generated files

Automated update of chart references (rancher-charts, partner-charts, rke2-charts)
and regenerated Dockerfiles and lock.yaml.

Automation: bump-pr-builder
Created-by: rancher-assets-bump-automation"

COMMIT_EXIT=0
commit_if_changed "$COMMIT_MSG" || COMMIT_EXIT=$?

if [ $COMMIT_EXIT -eq 1 ]; then
  summary "  ℹ️  No changes to commit (unexpected - should have been caught earlier)"
  git -C "$REPO_DIR" checkout -f "$TARGET_BRANCH"
  git -C "$REPO_DIR" branch -D "$BRANCH_NAME" || true
  exit 2
elif [ $COMMIT_EXIT -ne 0 ]; then
  summary "  ✗ Failed to commit changes (exit code: $COMMIT_EXIT)"
  git -C "$REPO_DIR" checkout -f "$TARGET_BRANCH"
  git -C "$REPO_DIR" branch -D "$BRANCH_NAME" || true
  exit 1
fi

summary "  ✓ Changes committed"

if [ "$DRY_RUN" = "true" ]; then
  summary "  ✓ Dry run - skipping push and PR creation"
  git -C "$REPO_DIR" checkout -f "$TARGET_BRANCH"
  exit 0
fi

# Push branch
log "  - Pushing branch \`$BRANCH_NAME\`"
if ! git -C "$REPO_DIR" push -u "$REMOTE" "$BRANCH_NAME"; then
  summary "  ✗ Failed to push branch"
  git -C "$REPO_DIR" checkout -f "$TARGET_BRANCH"
  exit 1
fi

summary "  ✓ Branch pushed"

# Create PR
log "  - Creating PR..."

PR_BODY="## Summary
Automated update of chart references and generated files.

## Changes
- Updated chart commit references in lock.yaml
- Regenerated Dockerfiles based on updated chart versions

## Verification
- [ ] Review chart version bumps in lock.yaml
- [ ] Verify Dockerfile changes are expected
- [ ] Check CI passes (smoke tests, build validation)

---
_This PR was automatically created by the bump-pr-builder automation._"

# Extract repo owner and name from SOURCE_REPO or fall back to parsing remote
REPO_OWNER_NAME="${SOURCE_REPO}"
if [ -z "$REPO_OWNER_NAME" ]; then
  REPO_OWNER_NAME=$(git -C "$REPO_DIR" remote get-url "$REMOTE" | sed -e 's|.*github.com[:/]||' -e 's|\.git$||')
fi

log "  - Running: gh pr create --repo $REPO_OWNER_NAME --base $TARGET_BRANCH --head $BRANCH_NAME"

# Try creating PR with label first
PR_OUTPUT=$(gh pr create \
  --repo "$REPO_OWNER_NAME" \
  --base "$TARGET_BRANCH" \
  --head "$BRANCH_NAME" \
  --title "Bump chart references and update generated files" \
  --body "$PR_BODY" \
  --label "status/auto-created" 2>&1)

PR_EXIT=$?

# If label fails, try without it
if [ $PR_EXIT -ne 0 ] && echo "$PR_OUTPUT" | grep -qi "label.*not found\|invalid.*label"; then
  log "  - Label 'status/auto-created' not found, retrying without label..."
  PR_OUTPUT=$(gh pr create \
    --repo "$REPO_OWNER_NAME" \
    --base "$TARGET_BRANCH" \
    --head "$BRANCH_NAME" \
    --title "Bump chart references and update generated files" \
    --body "$PR_BODY" 2>&1)
  PR_EXIT=$?
fi

if [ $PR_EXIT -eq 0 ]; then
  PR_URL=$(echo "$PR_OUTPUT" | tail -1)
  summary "  ✓ PR created: $PR_URL"
  summary ""
  summary "## Pull Request Created"
  summary "$PR_URL"
else
  summary "  ✗ Failed to create PR (exit code: $PR_EXIT)"
  summary ""
  summary "### Error Output:"
  summary '```'
  echo "$PR_OUTPUT" | while IFS= read -r line; do
    summary "$line"
  done
  summary '```'
  git -C "$REPO_DIR" checkout -f "$TARGET_BRANCH"
  exit 1
fi

# Return to target branch
git -C "$REPO_DIR" checkout -f "$TARGET_BRANCH"
