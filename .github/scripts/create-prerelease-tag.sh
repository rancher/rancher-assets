#!/bin/bash
set -e

VERSION="$1"
MAJOR="$2"
CURRENT_STABLE="$3"
CURRENT_PRERELEASE="$4"

git tag -a "$VERSION" -m "Auto pre-release $VERSION

Chart Major: $MAJOR
Triggered by: push to main

Current Stable: $CURRENT_STABLE
Current Prerelease: $CURRENT_PRERELEASE"

echo "Created tag: $VERSION"
