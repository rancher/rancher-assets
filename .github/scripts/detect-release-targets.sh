#!/bin/sh
# detect-release-targets.sh
#
# Determines which images to release for a push: one "<minor> <build_type>"
# pair per Rancher minor that has changed inputs (see detect-changed-dockerfiles.sh).
#
# A minor is a PROD release only when rancher-charts' prod commit moved for it in
# lock.yaml. partner-charts and rke2-charts share one upstream branch between prod
# and dev, so their movement (or a script/base image change) says nothing about a
# production release; those minors are DEV releases. A prod release for any other
# reason is done by pushing a prod CalVer tag by hand.
#
# A minor that is new in lock.yaml, or a missing lock.yaml at <from_ref>, is DEV:
# prod requires an observed rancher-charts move.
#
# Usage: detect-release-targets.sh <from_ref> <to_ref>
# Output: One "<minor> <build_type>" pair per line, e.g.
#           2.16 prod
#           2.15 dev
#
# Examples:
#   ./detect-release-targets.sh HEAD^ HEAD
#   ./detect-release-targets.sh ${{ github.event.before }} ${{ github.sha }}

set -e

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <from_ref> <to_ref>" >&2
    echo "Example: $0 HEAD^ HEAD" >&2
    exit 1
fi

FROM_REF="$1"
TO_REF="$2"

SCRIPT_DIR=$(dirname "$0")

echo "FROM_REF: $FROM_REF" >&2
echo "TO_REF: $TO_REF" >&2

DOCKERFILES=$("$SCRIPT_DIR/detect-changed-dockerfiles.sh" "$FROM_REF" "$TO_REF")

# Reduce changed Dockerfiles to the unique minors they belong to.
MINORS=""
for dockerfile in $DOCKERFILES; do
    eval "$("$SCRIPT_DIR/parse-dockerfile-name.sh" "$dockerfile")"
    if ! echo "$MINORS" | grep -qw "$RANCHER_MINOR"; then
        MINORS="$MINORS $RANCHER_MINOR"
    fi
done

for MINOR in $MINORS; do
    QUERY=".chart-versions.\"${MINOR}\".upstream-refs.prod.charts.commit"

    BEFORE_COMMIT=$(git show "${FROM_REF}:lock.yaml" 2>/dev/null | yq "$QUERY" || true)
    AFTER_COMMIT=$(git show "${TO_REF}:lock.yaml" 2>/dev/null | yq "$QUERY" || true)

    echo "$MINOR BEFORE_COMMIT: $BEFORE_COMMIT" >&2
    echo "$MINOR AFTER_COMMIT: $AFTER_COMMIT" >&2

    case "$BEFORE_COMMIT" in
        "" | null) BUILD_TYPE=dev ;;
        "$AFTER_COMMIT") BUILD_TYPE=dev ;;
        *) BUILD_TYPE=prod ;;
    esac

    echo "$MINOR $BUILD_TYPE"
done
