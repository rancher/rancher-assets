#!/bin/bash
set -e

# Script to create manual release tags
# Usage: ./scripts/create-manual-release.sh --bump={minor|patch} --release={prerelease|stable} [--major=v0] [--commit=SHA]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

BUMP_TYPE=""
RELEASE_TYPE=""
CHART_MAJOR=""
COMMIT_SHA=""

# Parse arguments
for arg in "$@"; do
  case $arg in
    --bump=*)
      BUMP_TYPE="${arg#*=}"
      shift
      ;;
    --release=*)
      RELEASE_TYPE="${arg#*=}"
      shift
      ;;
    --major=*)
      CHART_MAJOR="${arg#*=}"
      shift
      ;;
    --commit=*)
      COMMIT_SHA="${arg#*=}"
      shift
      ;;
    *)
      echo "Unknown argument: $arg"
      echo "Usage: $0 --bump={minor|patch} --release={prerelease|stable} [--major=v0] [--commit=SHA]"
      exit 1
      ;;
  esac
done

# Validate required arguments
if [ -z "$BUMP_TYPE" ]; then
  echo "Error: --bump is required"
  echo "Usage: $0 --bump={minor|patch} --release={prerelease|stable} [--major=v0] [--commit=SHA]"
  exit 1
fi

if [ -z "$RELEASE_TYPE" ]; then
  echo "Error: --release is required"
  echo "Usage: $0 --bump={minor|patch} --release={prerelease|stable} [--major=v0] [--commit=SHA]"
  exit 1
fi

if [ "$BUMP_TYPE" != "minor" ] && [ "$BUMP_TYPE" != "patch" ]; then
  echo "Error: --bump must be 'minor' or 'patch'"
  exit 1
fi

if [ "$RELEASE_TYPE" != "prerelease" ] && [ "$RELEASE_TYPE" != "stable" ]; then
  echo "Error: --release must be 'prerelease' or 'stable'"
  exit 1
fi

# Default to HEAD if no commit specified
if [ -z "$COMMIT_SHA" ]; then
  COMMIT_SHA=$(git rev-parse HEAD)
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Manual Release Tag Creator"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Bump Type: $BUMP_TYPE"
echo "Release Type: $RELEASE_TYPE"
if [ -n "$CHART_MAJOR" ]; then
  echo "Chart Major: $CHART_MAJOR"
else
  echo "Chart Major: ALL"
fi
echo "Commit: $COMMIT_SHA"
echo ""

# Determine chart majors
if [ -z "$CHART_MAJOR" ]; then
  echo "Releasing ALL active chart majors"
  MAJORS=$(yq eval '.chart-versions | keys | .[]' config.yaml | jq -R -s -c 'split("\n")[:-1]')
else
  echo "Releasing only $CHART_MAJOR"
  MAJORS="[\"$CHART_MAJOR\"]"
fi

# Always fetch latest versions branch
echo "Fetching latest versions branch..."
git fetch origin versions:versions

# Set up worktree for versions branch
VERSIONS_DIR=$(mktemp -d)
trap "rm -rf $VERSIONS_DIR" EXIT

git worktree add "$VERSIONS_DIR" versions >/dev/null 2>&1

# Plan releases
echo "Planning release version bumps..."
RELEASE_PLAN_OUTPUT=$(go run main.go plan-release \
  --versions-file="$VERSIONS_DIR/versions.yaml" \
  --type=manual \
  --majors="$MAJORS" \
  --bump="$BUMP_TYPE" \
  --release="$RELEASE_TYPE" \
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

  .github/scripts/create-release-tag.sh \
    "$VERSION" \
    "$COMMIT_SHA" \
    "$MAJOR" \
    "$RELEASE_TYPE" \
    "$BUMP_TYPE" \
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

  echo "Updating $MAJOR $RELEASE_TYPE to $NEW_VERSION"

  yq eval ".${MAJOR}.${RELEASE_TYPE}.tag = \"${NEW_VERSION}\"" -i versions.yaml
  yq eval ".${MAJOR}.${RELEASE_TYPE}.updated-at = \"${TIMESTAMP}\"" -i versions.yaml
done

git add versions.yaml

if ! git diff --cached --quiet; then
  git commit -m "Update versions after manual release

Release Type: $RELEASE_TYPE
Bump Type: $BUMP_TYPE
Chart Majors: $MAJORS
Commit: $COMMIT_SHA"

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
