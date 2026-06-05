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
echo "Auto Pre-release Tag Creator"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Detect changed majors
echo "Detecting changes in lock.yaml between $FROM_REF and $TO_REF..."
CHANGED_MAJORS_OUTPUT=$(go run main.go changed-majors --from="$FROM_REF" --to="$TO_REF" --verbose)
CHANGED_MAJORS=$(echo "$CHANGED_MAJORS_OUTPUT" | tail -1)

if [ "$CHANGED_MAJORS" = "[]" ]; then
  echo "No changes detected in lock.yaml"
  exit 0
fi

echo "Changed majors: $CHANGED_MAJORS"
echo ""

# Always fetch latest versions branch
echo "Fetching latest versions branch..."
git fetch origin versions:versions

# Set up worktree for versions branch
VERSIONS_DIR=$(mktemp -d)
trap "rm -rf $VERSIONS_DIR" EXIT

git worktree add "$VERSIONS_DIR" versions >/dev/null 2>&1

# Plan releases
echo "Planning pre-release version bumps..."
RELEASE_PLAN_OUTPUT=$(go run main.go plan-release \
  --versions-file="$VERSIONS_DIR/versions.yaml" \
  --type=auto \
  --changed-majors="$CHANGED_MAJORS" \
  --verbose)
RELEASE_PLAN=$(echo "$RELEASE_PLAN_OUTPUT" | tail -1)

git worktree remove "$VERSIONS_DIR" >/dev/null 2>&1

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
  MAJOR=$(echo "$release" | jq -r '.major')
  VERSION=$(echo "$release" | jq -r '.new_version')
  CURRENT_STABLE=$(echo "$release" | jq -r '.current_stable')
  CURRENT_PRERELEASE=$(echo "$release" | jq -r '.current_prerelease')

  echo ""
  echo "Creating tag $VERSION for $MAJOR at $COMMIT_SHA"

  .github/scripts/create-prerelease-tag.sh \
    "$VERSION" \
    "$COMMIT_SHA" \
    "$MAJOR" \
    "$CURRENT_STABLE" \
    "$CURRENT_PRERELEASE"

  git push origin "$VERSION"
  echo "✅ Pushed tag $VERSION"
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Updating versions branch..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Update versions branch using worktree
if ! git show-ref --verify --quiet refs/heads/versions; then
  git fetch origin versions:versions
fi

VERSIONS_UPDATE_DIR=$(mktemp -d)
trap "rm -rf $VERSIONS_UPDATE_DIR" EXIT

git worktree add "$VERSIONS_UPDATE_DIR" versions >/dev/null 2>&1

cd "$VERSIONS_UPDATE_DIR"

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "$RELEASE_PLAN" | jq -c '.[]' | while read -r release; do
  MAJOR=$(echo "$release" | jq -r '.major')
  NEW_VERSION=$(echo "$release" | jq -r '.new_version')

  echo "Updating $MAJOR prerelease to $NEW_VERSION"

  yq eval ".${MAJOR}.prerelease.tag = \"${NEW_VERSION}\"" -i versions.yaml
  yq eval ".${MAJOR}.prerelease.updated-at = \"${TIMESTAMP}\"" -i versions.yaml
done

git add versions.yaml

if ! git diff --cached --quiet; then
  git commit -m "Auto-update prerelease versions

Triggered by: commit $COMMIT_SHA
Changed majors: $CHANGED_MAJORS"

  git push origin versions
  echo "✅ Updated versions branch"
else
  echo "No changes to versions.yaml"
fi

cd "$REPO_ROOT"
git worktree remove "$VERSIONS_UPDATE_DIR" >/dev/null 2>&1

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ All tags created and versions updated"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Tags will trigger build workflow at:"
echo "https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/actions"
