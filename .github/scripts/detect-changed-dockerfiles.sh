#!/bin/sh
# detect-changed-dockerfiles.sh
#
# Detects which Dockerfiles changed between two git refs.
# Used by CI workflows to determine which images need to be built/released.
#
# Usage: detect-changed-dockerfiles.sh <from_ref> <to_ref>
# Output: One Dockerfile path per line (e.g., "dockerfiles/Dockerfile.2.14")
#
# Examples:
#   ./detect-changed-dockerfiles.sh HEAD^ HEAD
#   ./detect-changed-dockerfiles.sh origin/main HEAD
#   ./detect-changed-dockerfiles.sh ${{ github.event.before }} ${{ github.sha }}

set -e

# Validate arguments
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <from_ref> <to_ref>" >&2
    echo "Example: $0 HEAD^ HEAD" >&2
    exit 1
fi

FROM_REF="$1"
TO_REF="$2"

# Detect changed Dockerfiles
# --name-only: only show file names
# --diff-filter=ACMR: Added, Copied, Modified, Renamed (not Deleted)
# -- dockerfiles/: only look in dockerfiles directory
CHANGED=$(
    git diff --name-only --diff-filter=ACMR \
        "$FROM_REF..$TO_REF" -- dockerfiles/ | grep -E '^dockerfiles/Dockerfile\.' || true
)

# Create a Variable to store the Minors that Had changes
CHANGED_MINORS=""

for dockerfile in $CHANGED; do
    [ -z "$dockerfile" ] && continue

    # Reuse the existing parser instead of re-deriving the prefix/-dev-suffix
    # stripping here - it already knows the Dockerfile.X.XX[-dev] format and
    # sets $RANCHER_MINOR / $BUILD_TYPE.
    eval "$(./.github/scripts/parse-dockerfile-name.sh "$dockerfile")"

    # Dedup: only append if not already recorded.
    if ! echo "$CHANGED_MINORS" | grep -qw "$RANCHER_MINOR"; then
        CHANGED_MINORS="$CHANGED_MINORS $RANCHER_MINOR"
    fi
done

echo "$CHANGED_MINORS"
