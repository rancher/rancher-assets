#!/usr/bin/env bash
# Updates defaultAssetsImage in rancher/rancher build.yaml
#
# Required env vars:
#   TAG          - rancher-assets tag (e.g. v2.16-20260901T1200Z)
#   RANCHER_DIR  - path to rancher/rancher clone
#
# Exit codes:
#   0 - updated successfully
#   2 - no update needed (version already matches)
#   1 - error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_var TAG
require_rancher_dir

BUILD_YAML="$RANCHER_DIR/build.yaml"

if [ ! -f "$BUILD_YAML" ]; then
  echo "ERROR: build.yaml not found at $BUILD_YAML" >&2
  exit 1
fi

# Check if yq is available
if ! command -v yq >/dev/null 2>&1; then
  echo "ERROR: yq is required but not installed" >&2
  exit 1
fi

# Get current image value
CURRENT_IMAGE=$(yq eval '.defaultAssetsImage' "$BUILD_YAML")
NEW_IMAGE="rancher/rancher-assets:${TAG}"

log "  - Current: $CURRENT_IMAGE"
log "  - New:     $NEW_IMAGE"

# Check if update is needed
if [ "$CURRENT_IMAGE" = "$NEW_IMAGE" ]; then
  log "  ℹ️  Image already set to $NEW_IMAGE"
  exit 2
fi

# Update build.yaml
yq eval ".defaultAssetsImage = \"$NEW_IMAGE\"" -i "$BUILD_YAML"

log "  ✓ Updated defaultAssetsImage to $NEW_IMAGE"
exit 0
