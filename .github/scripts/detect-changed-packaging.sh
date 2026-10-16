#!/bin/sh
# detect-changed-packaging.sh
#
# Determines whether the PROD (non-dev) Dockerfile deserves a real release
# tag for reasons other than rancher-charts moving: a bumped base image,
# a fixed copy-charts.sh, or an edited prod Dockerfile template. These are
# this repo's own packaging inputs, not upstream chart commits - tracked
# globally in lock.yaml, not per Rancher minor, unlike detect-charts-release.sh.
#
# Usage: detect-changed-packaging.sh <from_ref> <to_ref>
# Output: "true" if base-image, copy-script-hash, or the prod template
#         changed; "false" otherwise.
#
# Examples:
#   ./detect-changed-packaging.sh HEAD^ HEAD
#   ./detect-changed-packaging.sh ${{ github.event.before }} ${{ github.sha }}

set -e

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <from_ref> <to_ref>" >&2
    echo "Example: $0 HEAD^ HEAD" >&2
    exit 1
fi

FROM_REF="$1"
TO_REF="$2"

echo "FROM_REF: $FROM_REF" >&2
echo "TO_REF: $TO_REF" >&2

BEFORE_LOCK=$(git show "${FROM_REF}:lock.yaml")
AFTER_LOCK=$(git show "${TO_REF}:lock.yaml")

CHANGED=false

for QUERY in \
    ".base-image.bci-base" \
    ".base-image.bci-micro" \
    ".copy-script-hash" \
    ".template-hashes.prod-template"
do
    BEFORE_VALUE=$(echo "$BEFORE_LOCK" | yq "$QUERY")
    AFTER_VALUE=$(echo "$AFTER_LOCK" | yq "$QUERY")

    echo "  $QUERY" >&2
    echo "    before: $BEFORE_VALUE" >&2
    echo "    after:  $AFTER_VALUE" >&2

    if [ "$BEFORE_VALUE" != "$AFTER_VALUE" ]; then
        echo "    -> changed" >&2
        CHANGED=true
    fi
done

echo "$CHANGED"
