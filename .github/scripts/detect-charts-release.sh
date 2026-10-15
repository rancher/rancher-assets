#!/bin/sh
# detect-charts-release.sh
#
# Determines whether the PROD (non-dev) Dockerfile for a Rancher minor
# deserves a real release tag, by checking lock.yaml directly instead of
# the rendered Dockerfile text. partner-charts and rke2-charts share the
# same upstream branch for both prod and dev refs, so their movement alone
# can make the prod Dockerfile's rendered text differ without rancher-charts
# (the canonical source) having moved at all. Called after
# detect-changed-dockerfiles.sh, which only tells you the Dockerfile text
# changed - not why.
#
# Usage: detect-charts-release.sh <from_ref> <to_ref> <minor>
# Output: "true" if rancher-charts' prod commit moved, "false" otherwise.
#
# Examples:
#   ./detect-charts-release.sh HEAD^ HEAD 2.16
#   ./detect-charts-release.sh ${{ github.event.before }} ${{ github.sha }} 2.16

set -e

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <from_ref> <to_ref> <minor>" >&2
    echo "Example: $0 HEAD^ HEAD 2.16" >&2
    exit 1
fi

FROM_REF="$1"
TO_REF="$2"
MINOR="$3"

echo "FROM_REF: "  ${FROM_REF}
echo "TO_REF: " ${TO_REF}
echo "MINOR: " ${MINOR}

QUERY=".chart-versions.\"${MINOR}\".upstream-refs.prod.charts.commit"

BEFORE_COMMIT=$(git show "${FROM_REF}:lock.yaml" | yq "$QUERY")
AFTER_COMMIT=$(git show "${TO_REF}:lock.yaml" | yq "$QUERY")

echo "BEFORE_COMMIT: " ${BEFORE_COMMIT}
echo "AFTER_COMMIT: " ${AFTER_COMMIT}

# TODO: also check base-image.bci-base/bci-micro, copy-script-hash, and
# template-hashes.prod-template - a legitimate BCI/script/template bump with
# no chart movement should still count as a real release, not get skipped.

if [ "$BEFORE_COMMIT" != "$AFTER_COMMIT" ]; then
    echo "true"
else
    echo "false"
fi
