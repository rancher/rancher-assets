#!/bin/sh
# generate-calver-tag.sh
#
# Generates a CalVer tag from version info and git commit.
# CalVer format: v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]
#
# Usage: generate-calver-tag.sh <rancher_minor> <build_type> <commit_sha>
# Output: Tag string (e.g., v2.14-20260724T1430Z-dev)
#
# Examples:
#   ./generate-calver-tag.sh 2.14 prod abc123
#   Output: v2.14-20260724T1430Z
#
#   ./generate-calver-tag.sh 2.15 dev def456
#   Output: v2.15-20260724T1431Z-dev

set -e

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <rancher_minor> <build_type> <commit_sha>" >&2
    echo "Example: $0 2.14 prod abc123" >&2
    exit 1
fi

RANCHER_MINOR="$1"
BUILD_TYPE="$2"
COMMIT_SHA="$3"

if [ "$BUILD_TYPE" != "prod" ] && [ "$BUILD_TYPE" != "dev" ]; then
    echo "Error: build_type must be 'prod' or 'dev', got: $BUILD_TYPE" >&2
    exit 1
fi

COMMIT_TIMESTAMP=$(git show -s --format=%ct "$COMMIT_SHA")

# GNU date (Linux/GitHub Actions) vs BSD date (macOS)
if date --version >/dev/null 2>&1; then
    CALVER_DATE=$(date -u -d "@$COMMIT_TIMESTAMP" '+%Y%m%dT%H%MZ')
else
    CALVER_DATE=$(date -u -r "$COMMIT_TIMESTAMP" '+%Y%m%dT%H%MZ')
fi

TAG="v${RANCHER_MINOR}-${CALVER_DATE}"

if [ "$BUILD_TYPE" = "dev" ]; then
    TAG="${TAG}-dev"
fi

echo "$TAG"
