#!/bin/sh
# detect-changed-dockerfiles.sh
#
# Detects which Dockerfiles changed between two git refs.
# Used by CI workflows to determine which images need to be built/released.
#
# Also flags every minor when lock.yaml's copy-script-hash changes. That hash
# tracks a script COPY'd into the image that the Dockerfile text does not
# reference, so a script change would otherwise never alter a rendered
# Dockerfile. Everything else in lock.yaml (upstream refs, base images,
# templates) is already rendered into the Dockerfiles and caught by the diff.
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

# Add a minor to CHANGED_MINORS if not already recorded.
add_minor() {
    if ! echo "$CHANGED_MINORS" | grep -qw "$1"; then
        CHANGED_MINORS="$CHANGED_MINORS $1"
    fi
}

for dockerfile in $CHANGED; do
    [ -z "$dockerfile" ] && continue

    # Reuse the existing parser instead of re-deriving the prefix/-dev-suffix
    # stripping here - it already knows the Dockerfile.X.XX[-dev] format and
    # sets $RANCHER_MINOR / $BUILD_TYPE.
    eval "$(./.github/scripts/parse-dockerfile-name.sh "$dockerfile")"

    add_minor "$RANCHER_MINOR"
done

# Script copied into the image isn't referenced in the Dockerfile text; its
# hash in lock.yaml is the only signal. Missing lock.yaml at a ref yields null.
HASH_QUERY='.["copy-script-hash"]'
FROM_HASH=$(git show "$FROM_REF:lock.yaml" 2>/dev/null | yq "$HASH_QUERY" || true)
TO_HASH=$(git show "$TO_REF:lock.yaml" 2>/dev/null | yq "$HASH_QUERY" || true)

if [ "$FROM_HASH" != "$TO_HASH" ]; then
    # copy-script-hash is global: it affects every minor.
    for minor in $(git show "$TO_REF:lock.yaml" | yq '.["chart-versions"] | keys | .[]'); do
        add_minor "$minor"
    done
fi

echo "$CHANGED_MINORS"
