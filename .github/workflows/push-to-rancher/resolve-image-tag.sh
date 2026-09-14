#!/usr/bin/env bash
# Resolves the rancher-assets image tag for a given git ref and rancher minor version.
# Used by push-to-rancher.yml for manual/workflow_call triggers.
#
# Usage:
#   resolve-image-tag.sh <git-ref> <rancher-minor>
#
# Arguments:
#   git-ref       - Commit SHA, tag, or branch to resolve
#   rancher-minor - Rancher minor version (e.g., 2.16)
#
# Output:
#   Prints the resolved image tag (e.g., v2.16-20260901T1200Z) to stdout
#
# Exit codes:
#   0 - Success
#   1 - No matching tag found

set -euo pipefail

if [ $# -ne 2 ]; then
  echo "Usage: $0 <git-ref> <rancher-minor>" >&2
  exit 1
fi

GIT_REF="$1"
MINOR="$2"

echo "Resolving image tag for:" >&2
echo "  Git ref: $GIT_REF" >&2
echo "  Rancher minor: $MINOR" >&2
echo "" >&2

# Find tags pointing to this git ref that match the expected version pattern
# Pattern: v2.16-20260901T1200Z or v2.16-20260901T1200Z-dev
MATCHING_TAGS=$(git tag --points-at "$GIT_REF" | grep -E "^v?${MINOR}-[0-9]{8}T[0-9]{6}Z(-dev)?$" || true)

if [ -z "$MATCHING_TAGS" ]; then
  echo "ERROR: No image tag found for commit $GIT_REF with version $MINOR" >&2
  echo "Expected tag format: v${MINOR}-YYYYMMDDTHHMMSSz[-dev]" >&2
  echo "" >&2
  echo "Available tags for this commit:" >&2
  git tag --points-at "$GIT_REF" || echo "  (none)" >&2
  exit 1
fi

# Prefer non-dev tags, otherwise take the first match
TAG=$(echo "$MATCHING_TAGS" | grep -v -- '-dev$' | head -1)
if [ -z "$TAG" ]; then
  TAG=$(echo "$MATCHING_TAGS" | head -1)
fi

echo "Resolved image tag: $TAG" >&2
echo "$TAG"
