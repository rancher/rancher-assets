#!/bin/bash
set -e

VERSION="$1"
COMMIT_SHA="$2"
MAJOR="$3"
CURRENT_STABLE="$4"
CURRENT_PRERELEASE="$5"

git tag -a "$VERSION" "$COMMIT_SHA" -m "Auto pre-release $VERSION

Chart Major: $MAJOR
Triggered by: push to main

Current Stable: $CURRENT_STABLE
Current Prerelease: $CURRENT_PRERELEASE"

echo "Created tag: $VERSION"
