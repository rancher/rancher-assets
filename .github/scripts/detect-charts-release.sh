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
# Usage: detect-charts-release.sh <from_ref> <to_ref> <minors>
#   <minors> is one or more minors separated by whitespace/newlines, as
#   emitted by detect-changed-dockerfiles.sh.
# Output: "true" if rancher-charts' prod commit moved for ANY of the given
#         minors, "false" otherwise.
#
# Examples:
#   ./detect-charts-release.sh HEAD^ HEAD 2.16
#   ./detect-charts-release.sh ${{ github.event.before }} ${{ github.sha }} 2.16
#   ./detect-charts-release.sh HEAD^ HEAD "$(./detect-changed-dockerfiles.sh HEAD^ HEAD)"

set -e

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <from_ref> <to_ref> <minors>" >&2
    echo "Example: $0 HEAD^ HEAD 2.16" >&2
    exit 1
fi

FROM_REF="$1"
TO_REF="$2"
MINORS="$3"

echo "FROM_REF: "  ${FROM_REF} >&2
echo "TO_REF: " ${TO_REF} >&2
echo "MINORS: " ${MINORS} >&2

RESULT=false

# Intentionally unquoted: split MINORS on whitespace/newlines
for MINOR in ${MINORS}; do
    QUERY=".chart-versions.\"${MINOR}\".upstream-refs.prod.charts.commit"

    BEFORE_COMMIT=$(git show "${FROM_REF}:lock.yaml" | yq "$QUERY")
    AFTER_COMMIT=$(git show "${TO_REF}:lock.yaml" | yq "$QUERY")

    echo "${MINOR} BEFORE_COMMIT: " ${BEFORE_COMMIT} >&2
    echo "${MINOR} AFTER_COMMIT: " ${AFTER_COMMIT} >&2

    if [ "$BEFORE_COMMIT" != "$AFTER_COMMIT" ]; then
        RESULT=true
    fi
done

echo "$RESULT"
