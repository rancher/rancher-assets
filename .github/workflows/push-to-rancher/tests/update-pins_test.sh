#!/usr/bin/env bash
# Tests of update-pins.sh against fixtures: no network, no rancher/rancher clone. Run it from anywhere:
#   .github/workflows/push-to-rancher/tests/update-pins_test.sh
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$HERE/../update-pins.sh"
failures=0

# A fresh copy of the fixtures as a "rancher/rancher clone"; its path is in $WORK.
fresh() {
  WORK=$(mktemp -d)
  cp "$HERE/fixtures/build.yaml" "$WORK/build.yaml"
  mkdir -p "$WORK/package"
  cp "$HERE/fixtures/package/Dockerfile" "$WORK/package/Dockerfile"
  REPORT="$WORK/report.md"
}

# run <tag> -> sets $EXIT
run() {
  EXIT=0
  TAG="$1" RANCHER_DIR="$WORK" INDEX_FILE="$HERE/fixtures/index.yaml" PINS_REPORT_FILE="$REPORT" \
    BASE_BUILD_YAML="${BASE_BUILD_YAML:-}" BASE_DOCKERFILE="${BASE_DOCKERFILE:-}" \
    bash "$SCRIPT" >"$WORK/out.log" 2>&1 || EXIT=$?
}

check() { # check <name> <expected> <actual>
  if [ "$2" = "$3" ]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1"
    echo "       expected: $2"
    echo "       actual:   $3"
    failures=$((failures + 1))
  fi
}

val() { yq eval ".$1" "$WORK/build.yaml"; }

# 1. the 2.16 line takes the first entry of major 111, never the 112 that comes first in the list
fresh; run v2.16-20261120T1935Z-dev
check "2.16 dev image: exit 0" 0 "$EXIT"
check "webhook is the first 111 entry, not the 2.17 one" "111.0.1+up0.12.1-rc.22" "$(val webhookVersion)"
check "remotedialer-proxy is the first 111 entry" "111.0.1+up0.9.1-rc.5" "$(val remoteDialerProxyVersion)"
check "fleet is the final, not the rc" "111.0.1+up0.17.0" "$(val fleetVersion)"
check "turtles is the final, not the rc" "111.0.0+up0.28.0" "$(val turtlesVersion)"
check "cluster-api image comes from the turtles entry" "rancher/cluster-api-controller:v1.14.4_turtles.3" "$(val clusterAPIControllerImage)"
check "aks operator is the newest 111" "111.0.0+up1.16.0-rc.8" "$(val aksOperatorVersion)"
check "system-upgrade-controller chart version in package/Dockerfile" "ENV CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION=111.0.2" "$(grep '^ENV CATTLE_SYSTEM_UPGRADE' "$WORK/package/Dockerfile")"
check "other Dockerfile lines are untouched" "ENV CATTLE_OTHER=unchanged" "$(grep '^ENV CATTLE_OTHER' "$WORK/package/Dockerfile")"

# 2. what must not change
check "a chart with no entry for the major keeps its pin" "111.0.0+up1.16.0-rc.6" "$(val eksOperatorVersion)"
check "a key that the branch does not have is not added" "null" "$(val gkeOperatorVersion)"
check "the image tag is not touched" "rancher/rancher-assets:v2.16-20261120T1507Z-dev" "$(val defaultAssetsImage)"
check "comments of build.yaml are kept" "3" "$(grep -c '^# ' "$WORK/build.yaml")"

# 3. the report says what moved and what was skipped
check "report lists a moved pin" 1 "$(grep -c 'webhookVersion.*111.0.1+up0.12.1-rc.21.*111.0.1+up0.12.1-rc.22' "$REPORT")"
check "report lists the skipped chart" 1 "$(grep -c 'eksOperatorVersion.*rancher-eks-operator' "$REPORT")"

# 4. a second run has nothing to do
run v2.16-20261120T1935Z-dev
check "second run: nothing to update, exit 2" 2 "$EXIT"

# 5. the major follows the minor of the tag
fresh; run v2.17-20261201T1000Z-dev
check "2.17 dev image: webhook is the 112 entry" "112.0.0+up0.13.0-rc.11" "$(val webhookVersion)"

# 6. the guard: only dev images of a branch on the staging catalog
fresh; run v2.16-20261120T1935Z
check "not a dev image: exit 3" 3 "$EXIT"
check "not a dev image: pins untouched" "111.0.1+up0.12.1-rc.21" "$(val webhookVersion)"
fresh; yq eval -i '.chartDefaultBranch = "dev-v2.16"' "$WORK/build.yaml"; run v2.16-20261120T1935Z-dev
check "branch on the old system: exit 3" 3 "$EXIT"
check "branch on the old system: pins untouched" "111.0.1+up0.12.1-rc.21" "$(val webhookVersion)"

# 7. a branch that already has the pins of an earlier update (the PR is updated, not created): nothing is changed, and
#    the report still says what moved against the base branch, which is what the PR is about
fresh; run v2.16-20261120T1935Z-dev
BASE_BUILD_YAML="$HERE/fixtures/build.yaml" BASE_DOCKERFILE="$HERE/fixtures/package/Dockerfile" run v2.16-20261120T1935Z-dev
check "already updated branch: nothing to change, exit 2" 2 "$EXIT"
check "...but the report lists a pin moved against the base" 1 "$(grep -c 'webhookVersion.*111.0.1+up0.12.1-rc.21.*111.0.1+up0.12.1-rc.22' "$REPORT")"
check "...and the system-upgrade-controller version against the base" 1 "$(grep -c 'CATTLE_SYSTEM_UPGRADE_CONTROLLER_CHART_VERSION.*111.0.1.*111.0.2' "$REPORT")"
check "...and the cluster-api image against the base" 1 "$(grep -c 'clusterAPIControllerImage.*turtles.1.*turtles.3' "$REPORT")"
check "...and a pin that did not move against the base is not listed" 0 "$(grep -c 'aksOperatorVersion.*111.0.0+up1.16.0-rc.8.*111.0.0+up1.16.0-rc.8' "$REPORT")"
BASE_BUILD_YAML=""; BASE_DOCKERFILE=""

if [ "$failures" -ne 0 ]; then
  echo "$failures check(s) failed"
  exit 1
fi
echo "all checks passed"
