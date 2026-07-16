#!/bin/bash
set -e

# Script to create auto pre-release tags based on lock.yaml changes
# Usage: ./scripts/create-auto-prerelease.sh [--from=REF] [--to=REF]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

FROM_REF="HEAD^"
TO_REF="HEAD"

# Parse arguments
for arg in "$@"; do
  case $arg in
    --from=*)
      FROM_REF="${arg#*=}"
      shift
      ;;
    --to=*)
      TO_REF="${arg#*=}"
      shift
      ;;
    *)
      echo "Unknown argument: $arg"
      echo "Usage: $0 [--from=REF] [--to=REF]"
      exit 1
      ;;
  esac
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Auto Pre-release Tag Creator (CalVer)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Detect changed Rancher minors
echo "Detecting changes in lock.yaml between $FROM_REF and $TO_REF..."
CHANGED_MINORS=$(go run main.go changed-minors --from="$FROM_REF" --to="$TO_REF")

if [ "$CHANGED_MINORS" = "[]" ]; then
  echo "No changes detected in lock.yaml"
  exit 0
fi

echo "Changed Rancher minors: $CHANGED_MINORS"
echo ""

# Plan releases (CalVer - generates timestamps automatically)
echo "Planning CalVer pre-release versions..."
RELEASE_PLAN=$(go run main.go plan-release \
  --type=auto \
  --changed-minors="$CHANGED_MINORS")

echo ""
echo "Release Plan:"
echo "$RELEASE_PLAN" | jq .
echo ""

# Confirm with user
read -p "Create and push these tags? [y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted."
  exit 1
fi

# Get commit SHA to tag
COMMIT_SHA=$(git rev-parse "$TO_REF")

# Create and push tags
echo ""
echo "Creating tags..."
echo "$RELEASE_PLAN" | jq -c '.[]' | while read -r release; do
  MINOR=$(echo "$release" | jq -r '.rancher_minor')
  VERSION=$(echo "$release" | jq -r '.version')
  RTYPE=$(echo "$release" | jq -r '.release_type')

  echo ""
  echo "Creating tag $VERSION for Rancher $MINOR ($RTYPE) at $COMMIT_SHA"

  # Create annotated tag
  git tag -a "$VERSION" "$COMMIT_SHA" -m "Auto pre-release: $VERSION

Rancher Minor: $MINOR
Triggered by: commit $COMMIT_SHA
Changed minors: $CHANGED_MINORS"

  # Push tag
  git push origin "$VERSION"
  echo "✅ Pushed tag $VERSION"
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ All tags created successfully"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Tags will trigger build workflow at:"
echo "https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/actions"
