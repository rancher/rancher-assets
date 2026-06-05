#!/bin/bash
set -e

VERSION="$1"
MAJOR="$2"
RELEASE_TYPE="$3"
BUMP_TYPE="$4"
CURRENT_STABLE="$5"
CURRENT_PRERELEASE="$6"

git tag -a "$VERSION" -m "Release $VERSION

Chart Major: $MAJOR
Release Type: $RELEASE_TYPE
Bump Type: $BUMP_TYPE

Current Stable: $CURRENT_STABLE
Current Prerelease: $CURRENT_PRERELEASE"

echo "Created tag: $VERSION"
