#!/usr/bin/env bash
# Updates the chart pins of rancher/rancher (build.yaml and package/Dockerfile) to the newest staged version of each
# chart for the Rancher line of the image, read from the index.yaml of the charts commit that is inside the image.
#
# It only applies to a dev image (tag ending in -dev) and to a branch whose chartDefaultBranch is staging, the new
# catalog. For anything else it does nothing and exits with 3.
#
# "Newest" is the first entry of the chart in the index whose major is the one of the line: the index is ordered by
# cargueiro, newest first, across every Rancher line, so the first entry can be a version of the next line (the 2.17
# webhook comes before the 2.16 one). Chart versions are 1XY.Z.W+upA.B.C, where 1XY follows the Rancher minor (2.10 is
# 105 ... 2.16 is 111, 2.17 is 112: versions.majors of rules.yaml in rancher/cargueiro). Semver is not used on purpose:
# it ignores what comes after the +, so two release candidates of the same upstream version compare equal.
#
# Required env vars:
#   TAG          - rancher-assets tag (e.g. v2.16-20261120T1935Z-dev)
#   RANCHER_DIR  - path to rancher/rancher clone
#
# Optional env vars:
#   PINS_REPORT_FILE - a file to write what was moved and what was skipped to, as markdown
#   SOURCE_REPO      - repo of the tag, to read its lock.yaml (default rancher/rancher-assets)
#   CHARTS_REPO      - the charts repository (default rancher/charts)
#   INDEX_FILE       - use this index.yaml and do not read the lock.yaml or the network (for tests)
#
# Exit codes:
#   0 - updated
#   2 - no update needed (the pins are the newest already)
#   3 - not applicable (not a dev image, or the branch is not on the staging catalog)
#   1 - error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/common.sh"

require_var TAG
require_rancher_dir

BUILD_YAML="$RANCHER_DIR/build.yaml"
DOCKERFILE="$RANCHER_DIR/package/Dockerfile"
REPORT_FILE="${PINS_REPORT_FILE:-/dev/null}"
SOURCE_REPO="${SOURCE_REPO:-rancher/rancher-assets}"
CHARTS_REPO="${CHARTS_REPO:-rancher/charts}"

if [ ! -f "$BUILD_YAML" ]; then
  echo "ERROR: build.yaml not found at $BUILD_YAML" >&2
  exit 1
fi
if ! command -v yq >/dev/null 2>&1; then
  echo "ERROR: yq is required but not installed" >&2
  exit 1
fi

# Guard: only dev images of a branch on the staging catalog
if [[ "$TAG" != *-dev ]]; then
  log "  ℹ️  $TAG is not a dev image: the chart pins are not changed"
  exit 3
fi
CHART_BRANCH=$(yq eval '.chartDefaultBranch' "$BUILD_YAML")
if [ "$CHART_BRANCH" != "staging" ]; then
  log "  ℹ️  chartDefaultBranch is '$CHART_BRANCH', not staging: the chart pins are not changed"
  exit 3
fi

MINOR=$(get_tag_minor_version "$TAG")
if [ -z "$MINOR" ]; then
  echo "ERROR: could not read the minor version from the tag $TAG" >&2
  exit 1
fi
MAJOR=$((95 + ${MINOR#*.}))

# The index.yaml of the charts commit that the image was built from
CHARTS_COMMIT="(given)"
if [ -n "${INDEX_FILE:-}" ]; then
  INDEX="$INDEX_FILE"
else
  LOCK=$(curl -fsSL "https://raw.githubusercontent.com/${SOURCE_REPO}/${TAG}/lock.yaml") || {
    echo "ERROR: could not read lock.yaml of $TAG in $SOURCE_REPO" >&2
    exit 1
  }
  CHARTS_COMMIT=$(MINOR="$MINOR" yq eval '.chart-versions[strenv(MINOR)].upstream-refs.dev.charts.commit' - <<<"$LOCK")
  if [ -z "$CHARTS_COMMIT" ] || [ "$CHARTS_COMMIT" = "null" ]; then
    echo "ERROR: lock.yaml of $TAG has no dev charts commit for $MINOR" >&2
    exit 1
  fi
  INDEX=$(mktemp)
  trap 'rm -f "$INDEX"' EXIT
  curl -fsSL "https://raw.githubusercontent.com/${CHARTS_REPO}/${CHARTS_COMMIT}/index.yaml" -o "$INDEX" || {
    echo "ERROR: could not read index.yaml of $CHARTS_REPO at $CHARTS_COMMIT" >&2
    exit 1
  }
fi

log "  - Pins for the line $MINOR (major $MAJOR), from the index of $CHARTS_REPO at $CHARTS_COMMIT"

# The first entry of the chart whose major is the one of the line; empty when there is none. yq picks the first
# entry itself: a "| head -1" would make yq die of SIGPIPE, which pipefail turns into a failure.
latest_version() {
  local version
  version=$(CHART="$1" MAJOR="$MAJOR" yq eval \
    '.entries[strenv(CHART)] | map(select(.version | test("^" + strenv(MAJOR) + "\\."))) | .[0].version' "$INDEX")
  [ "$version" = "null" ] && version=""
  echo "$version"
}

CHANGED=0
MOVED=()
SKIPPED=()

# set_pin <key in build.yaml> <chart name>; prints the new version of the chart on stdout
set_pin() {
  local key="$1" chart="$2" current new
  current=$(KEY="$key" yq eval '.[strenv(KEY)]' "$BUILD_YAML")
  if [ "$current" = "null" ]; then
    return 0 # this branch does not have the pin
  fi
  new=$(latest_version "$chart")
  if [ -z "$new" ]; then
    SKIPPED+=("- \`$key\`: the index has no \`$chart\` of the line (major $MAJOR), so \`$current\` stays")
    return 0
  fi
  if [ "$current" != "$new" ]; then
    KEY="$key" NEW="$new" yq eval -i '.[strenv(KEY)] = strenv(NEW)' "$BUILD_YAML"
    MOVED+=("- \`$key\`: \`$current\` → \`$new\`")
    CHANGED=1
  fi
}

set_pin fleetVersion fleet
set_pin remoteDialerProxyVersion remotedialer-proxy
set_pin turtlesVersion rancher-turtles
set_pin webhookVersion rancher-webhook
set_pin aksOperatorVersion rancher-aks-operator
set_pin eksOperatorVersion rancher-eks-operator
set_pin gkeOperatorVersion rancher-gke-operator

# The cluster-api controller image is the one that the turtles chart of the line lists with dependency: cluster-api
CAPI_CURRENT=$(yq eval '.clusterAPIControllerImage' "$BUILD_YAML")
TURTLES=$(latest_version rancher-turtles)
if [ "$CAPI_CURRENT" != "null" ] && [ -n "$TURTLES" ]; then
  CAPI_NEW=$(CHART=rancher-turtles VERSION="$TURTLES" yq eval \
    '.entries[strenv(CHART)][] | select(.version == strenv(VERSION)) | .annotations["helm.sh/images"]' "$INDEX" |
    yq eval 'map(select(.dependency == "cluster-api")) | .[0].image' -)
  [ "$CAPI_NEW" = "null" ] && CAPI_NEW=""
  if [ -n "$CAPI_NEW" ] && [ "$CAPI_NEW" != "$CAPI_CURRENT" ]; then
    IMAGE="$CAPI_NEW" yq eval -i '.clusterAPIControllerImage = strenv(IMAGE)' "$BUILD_YAML"
    MOVED+=("- \`clusterAPIControllerImage\`: \`$CAPI_CURRENT\` → \`$CAPI_NEW\` (from the turtles chart \`$TURTLES\`)")
    CHANGED=1
  fi
fi

# The system-upgrade-controller chart version is not in build.yaml: it is an ENV of package/Dockerfile
SUC_LINE='ENV CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION='
if [ -f "$DOCKERFILE" ] && grep -q "^${SUC_LINE}" "$DOCKERFILE"; then
  SUC_CURRENT=$(grep "^${SUC_LINE}" "$DOCKERFILE" | head -1 | cut -d= -f2)
  SUC_NEW=$(latest_version system-upgrade-controller)
  if [ -z "$SUC_NEW" ]; then
    SKIPPED+=("- \`CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION\`: the index has no \`system-upgrade-controller\` of the line (major $MAJOR), so \`$SUC_CURRENT\` stays")
  elif [ "$SUC_CURRENT" != "$SUC_NEW" ]; then
    sed -i "s|^${SUC_LINE}.*|${SUC_LINE}${SUC_NEW}|" "$DOCKERFILE"
    MOVED+=("- \`CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION\` (\`package/Dockerfile\`): \`$SUC_CURRENT\` → \`$SUC_NEW\`")
    CHANGED=1
  fi
fi

{
  echo "Charts commit of the image: \`$CHARTS_COMMIT\`. The pins are the first entry of the chart in its \`index.yaml\` with major $MAJOR (the line $MINOR)."
  if [ ${#MOVED[@]} -gt 0 ]; then
    echo
    echo "Moved:"
    printf '%s\n' "${MOVED[@]}"
  fi
  if [ ${#SKIPPED[@]} -gt 0 ]; then
    echo
    echo "Left as they were:"
    printf '%s\n' "${SKIPPED[@]}"
  fi
} >"$REPORT_FILE"

if [ "$CHANGED" -eq 0 ]; then
  log "  ℹ️  The chart pins are the newest already"
  exit 2
fi

for line in "${MOVED[@]}"; do log "  ✓ $line"; done
for line in "${SKIPPED[@]:-}"; do [ -n "$line" ] && log "  ⚠️  $line"; done
exit 0
