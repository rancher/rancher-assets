#!/usr/bin/env bash
# Shared setup for push-to-rancher scripts. Source this file: source "$(dirname "$0")/common.sh"

# Determine ASSETS_DIR (rancher-assets root) from this script's location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="${ASSETS_DIR:-$(cd "$SCRIPT_DIR/../../.." && pwd)}"

# Required: path to a local rancher/rancher clone
RANCHER_DIR="${RANCHER_DIR:-}"

# Remote name for rancher/rancher in RANCHER_DIR (may differ locally if using a fork)
RANCHER_REMOTE="${RANCHER_REMOTE:-origin}"

# Skip git commits, push, and PR creation when true
DRY_RUN="${DRY_RUN:-false}"

# Version-to-branch mapping
# Update this when:
# - A new release branch is created (move main to next version, add release/vX.Y)
# - EOL versions are dropped (remove the mapping)
#
# Format: "VERSION:branch1,branch2,..."
# - VERSION is the minor version without 'v' prefix (e.g., "2.16")
# - Branches are comma-separated, no spaces
#
# Example lifecycle:
#   Initially: "2.16:main"
#   After release/v2.16 is cut: "2.17:main" and "2.16:release/v2.16"
#   After release/v2.17 is cut: "2.18:main" and "2.17:release/v2.17,2.16:release/v2.16"
#
declare -A VERSION_BRANCH_MAP=(
  ["2.16"]="main"
)

# Extract minor version from TAG (e.g., v2.16-20260901T1200Z -> 2.16)
get_tag_minor_version() {
  local tag="$1"
  echo "$tag" | sed -nE 's/^v?([0-9]+\.[0-9]+)-.*/\1/p'
}

# Get target branches for a given tag based on VERSION_BRANCH_MAP
# Returns space-separated list of branches
filter_branches_for_tag() {
  local tag="$1"

  # Check for override first (for testing/special cases)
  if [ -n "$RANCHER_BRANCHES_OVERRIDE" ]; then
    # Convert to space-separated, handling both comma and space separators
    echo "$RANCHER_BRANCHES_OVERRIDE" | tr ',' ' '
    return
  fi

  local minor_version
  minor_version=$(get_tag_minor_version "$tag")

  if [ -z "$minor_version" ]; then
    echo "ERROR: Could not extract minor version from tag: $tag" >&2
    echo "Expected format: v2.16-20260901T1200Z" >&2
    exit 1
  fi

  # Check if version exists in map
  if [ -z "${VERSION_BRANCH_MAP[$minor_version]:-}" ]; then
    echo "ERROR: No branch mapping found for version $minor_version" >&2
    echo "Available mappings:" >&2
    for version in "${!VERSION_BRANCH_MAP[@]}"; do
      echo "  $version -> ${VERSION_BRANCH_MAP[$version]}" >&2
    done
    echo "" >&2
    echo "Update VERSION_BRANCH_MAP in common.sh to add this version" >&2
    exit 1
  fi

  # Convert comma-separated branches to space-separated
  echo "${VERSION_BRANCH_MAP[$minor_version]}" | tr ',' ' '
}


# RANCHER_BRANCHES_OVERRIDE allows runtime override (for testing/special cases)
# Format: space or comma-separated list of branches
# This bypasses the VERSION_BRANCH_MAP and uses explicit branches instead
export RANCHER_BRANCHES_OVERRIDE="${RANCHER_BRANCHES_OVERRIDE:-}"

# Docker registry to validate image existence
IMAGE_REGISTRY="${IMAGE_REGISTRY:-docker.io}"
IMAGE_REPO="${IMAGE_REPO:-rancher/rancher-assets}"

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

require_rancher_dir() {
  require_var RANCHER_DIR
  if [ ! -d "$RANCHER_DIR" ]; then
    echo "ERROR: RANCHER_DIR '$RANCHER_DIR' does not exist" >&2
    exit 1
  fi
}

# Validate that the rancher-assets image exists in the registry
validate_image_exists() {
  local tag="$1"
  local validation_failed=0

  summary ""
  summary "## Image Validation"

  local full_image="${IMAGE_REGISTRY}/${IMAGE_REPO}:${tag}"
  summary "- **Docker Hub**: \`${full_image}\`"

  # Check if docker is available
  if ! command -v docker >/dev/null 2>&1; then
    summary "  ✗ Docker command not found"
    summary ""
    summary "### Error Details:"
    summary '```'
    summary "docker command is not available in PATH"
    summary "This workflow requires docker to validate image existence"
    summary '```'
    return 1
  fi

  # Try to inspect the manifest with detailed error output
  local manifest_output
  manifest_output=$(docker manifest inspect "$full_image" 2>&1)
  local manifest_exit=$?

  if [ $manifest_exit -eq 0 ]; then
    summary "  ✓ Image found"
  else
    summary "  ✗ Image NOT found or inaccessible"
    summary ""
    summary "### Error Details:"
    summary '```'
    echo "$manifest_output" | head -20 | while IFS= read -r line; do
      summary "$line"
    done
    summary '```'
    validation_failed=1
  fi

  if [ $validation_failed -eq 1 ]; then
    return 1
  fi

  summary ""
  summary "✅ Image validation passed"
  return 0
}

# Commit all changes in RANCHER_DIR if any exist. Returns 1 if no changes, 0 on success.
commit_if_changed() {
  local message="$1"
  if git -C "$RANCHER_DIR" diff --quiet --exit-code && [ -z "$(git -C "$RANCHER_DIR" status --porcelain)" ]; then
    return 1
  fi

  if ! git -C "$RANCHER_DIR" add . 2>&1; then
    echo "ERROR: Failed to stage changes" >&2
    return 2
  fi

  if ! git -C "$RANCHER_DIR" commit -m "$message" 2>&1; then
    echo "ERROR: Failed to create commit" >&2
    return 2
  fi

  return 0
}
