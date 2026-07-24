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
git diff --name-only --diff-filter=ACMR "$FROM_REF..$TO_REF" -- dockerfiles/ \
    | grep -E '^dockerfiles/Dockerfile\.' \
    || true  # Don't fail if no matches (grep returns 1 when no matches)

# Note: Output is one path per line, ready for iteration in workflows
