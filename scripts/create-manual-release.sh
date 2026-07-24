#!/bin/bash
set -e

# Script to create manual release tags using CalVer
# Usage: ./scripts/create-manual-release.sh --release={prerelease|stable} [--minor=2.15] [--commit=SHA]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

RELEASE_TYPE=""
RANCHER_MINOR=""
COMMIT_SHA=""

# Parse arguments
for arg in "$@"; do
  case $arg in
    --release=*)
      RELEASE_TYPE="${arg#*=}"
      shift
      ;;
    --minor=*)
      RANCHER_MINOR="${arg#*=}"
      shift
      ;;
    --commit=*)
      COMMIT_SHA="${arg#*=}"
      shift
      ;;
    *)
      echo "Unknown argument: $arg"
      echo "Usage: $0 --release={prerelease|stable} [--minor=2.15] [--commit=SHA]"
      exit 1
      ;;
  esac
done

# Validate required arguments
if [ -z "$RELEASE_TYPE" ]; then
  echo "Error: --release is required"
  echo "Usage: $0 --release={prerelease|stable} [--minor=2.15] [--commit=SHA]"
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
echo "Manual Release Tag Creator (CalVer)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Release Type: $RELEASE_TYPE"
if [ -n "$RANCHER_MINOR" ]; then
  echo "Rancher Minor: $RANCHER_MINOR"
else
  echo "Rancher Minor: ALL"
fi
echo "Commit: $COMMIT_SHA"
echo ""

# Determine Rancher minors
if [ -z "$RANCHER_MINOR" ]; then
  echo "Releasing ALL active Rancher minors"
  MINORS=$(yq eval '.chart-versions | keys | .[]' config.yaml | jq -R -s -c 'split("\n")[:-1]')
else
  echo "Releasing only $RANCHER_MINOR"
  MINORS="[\"$RANCHER_MINOR\"]"
fi

# Plan releases (CalVer - generates timestamps automatically)
echo "Planning CalVer release versions..."
RELEASE_PLAN=$(go run main.go plan-release \
  --type=manual \
  --minors="$MINORS" \
  --release="$RELEASE_TYPE")

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
  MINOR=$(echo "$release" | jq -r '.rancher_minor')
  VERSION=$(echo "$release" | jq -r '.version')
  RTYPE=$(echo "$release" | jq -r '.release_type')

  echo ""
  echo "Creating tag $VERSION for Rancher $MINOR ($RTYPE) at $COMMIT_SHA"

  # Create annotated tag
  git tag -a "$VERSION" "$COMMIT_SHA" -m "Manual $RTYPE release: $VERSION

Rancher Minor: $MINOR
Release Type: $RTYPE
Commit: $COMMIT_SHA"

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
