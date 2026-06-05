#!/bin/bash
set -e

VERSION="$1"
COMMIT_SHA="$2"
MAJOR="$3"
RELEASE_TYPE="$4"
BUMP_TYPE="$5"
CURRENT_STABLE="$6"
CURRENT_PRERELEASE="$7"

git tag -a "$VERSION" "$COMMIT_SHA" -m "Release $VERSION

Chart Major: $MAJOR
Release Type: $RELEASE_TYPE
Bump Type: $BUMP_TYPE

Current Stable: $CURRENT_STABLE
Current Prerelease: $CURRENT_PRERELEASE"

echo "Created tag: $VERSION"
