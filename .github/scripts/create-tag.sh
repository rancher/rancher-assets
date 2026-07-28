#!/bin/sh
# create-tag.sh
#
# Creates an annotated git tag with metadata.
# Used by CI workflows to tag releases.
#
# Usage: create-tag.sh <tag_name> <commit_sha> <dockerfile_path>
# Example: create-tag.sh v2.14-20260724T1430Z abc123 dockerfiles/Dockerfile.2.14
#
# Note: This script only creates the tag locally. The caller must push it.

set -e

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <tag_name> <commit_sha> <dockerfile_path>" >&2
    echo "Example: $0 v2.14-20260724T1430Z abc123 dockerfiles/Dockerfile.2.14" >&2
    exit 1
fi

TAG_NAME="$1"
COMMIT_SHA="$2"
DOCKERFILE_PATH="$3"

# Build tag message
TAG_MESSAGE="Auto-release for ${DOCKERFILE_PATH} from commit ${COMMIT_SHA}"

# Create annotated tag
# If tag already exists, this will fail (which is what we want)
git tag -a "$TAG_NAME" "$COMMIT_SHA" -m "$TAG_MESSAGE"

echo "Created tag: $TAG_NAME at $COMMIT_SHA" >&2
