#!/bin/sh
# build-image.sh
#
# Builds a Docker image for a specific Dockerfile.
# Used by both GitHub Actions workflows AND the Makefile.
# Also can be used directly by developers for local testing.
#
# Usage: build-image.sh <dockerfile_path> <image_tag> [--push]
# Example: build-image.sh dockerfiles/Dockerfile.2.14 v2.14-20260724T1430Z
#
# Environment variables (optional, auto-detected if not set):
#   RANCHER_MINOR         - Rancher version (e.g., "2.14")
#   GIT_COMMIT            - Git commit SHA
#   BUILD_DATE            - Build timestamp
#   BUILD_URL             - CI run URL
#   GITHUB_REPOSITORY_OWNER - For image tag (defaults to $USER)
#   TARGET_PLATFORMS      - Platforms to build (default: linux/amd64,linux/arm64)

set -e

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <dockerfile_path> <image_tag> [--push]" >&2
    echo "Example: $0 dockerfiles/Dockerfile.2.14 v2.14-20260724T1430Z" >&2
    exit 1
fi

DOCKERFILE_PATH="$1"
IMAGE_TAG="$2"
PUSH_FLAG=""

# Check for --push flag
if [ "$#" -eq 3 ] && [ "$3" = "--push" ]; then
    PUSH_FLAG="--push"
else
    # Load image locally for testing
    PUSH_FLAG="--load"
fi

# Validate Dockerfile exists
if [ ! -f "$DOCKERFILE_PATH" ]; then
    echo "Error: Dockerfile not found: $DOCKERFILE_PATH" >&2
    exit 1
fi

# Extract RANCHER_MINOR from dockerfile path if not already set
if [ -z "$RANCHER_MINOR" ]; then
    FILENAME=$(basename "$DOCKERFILE_PATH")
    # Remove Dockerfile. prefix and -dev suffix
    RANCHER_MINOR=$(echo "$FILENAME" | sed 's/^Dockerfile\.//' | sed 's/-dev$//')
fi

# Set defaults for environment variables
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo 'unknown')}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
BUILD_URL="${BUILD_URL:-local}"
GITHUB_REPOSITORY_OWNER="${GITHUB_REPOSITORY_OWNER:-${USER}}"
TARGET_PLATFORMS="${TARGET_PLATFORMS:-linux/amd64,linux/arm64}"

# Get TARGET_BRANCH from config.yaml
if command -v yq >/dev/null 2>&1 && [ -f config.yaml ]; then
    TARGET_BRANCH=$(yq eval ".chart-versions.\"${RANCHER_MINOR}\".rancher-branch" config.yaml 2>/dev/null || echo 'unknown')
else
    TARGET_BRANCH="unknown"
fi

# Build image name
IMAGE_REPO="ghcr.io/${GITHUB_REPOSITORY_OWNER}/rancher-assets"
FULL_IMAGE_TAG="${IMAGE_REPO}:${IMAGE_TAG}"

# Determine build type for logging
BUILD_TYPE="prod"
if echo "$DOCKERFILE_PATH" | grep -q -- '-dev$'; then
    BUILD_TYPE="dev"
fi

# Log build info
echo "Building Rancher ${RANCHER_MINOR} version ${IMAGE_TAG} (${BUILD_TYPE})"
echo "  Dockerfile: ${DOCKERFILE_PATH}"
echo "  Image: ${FULL_IMAGE_TAG}"
echo "  Platforms: ${TARGET_PLATFORMS}"
echo ""

# Build the image
docker buildx build \
    --file "$DOCKERFILE_PATH" \
    --platform "$TARGET_PLATFORMS" \
    --build-arg VERSION="$IMAGE_TAG" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    --build-arg TARGET_BRANCH="$TARGET_BRANCH" \
    --build-arg BUILD_URL="$BUILD_URL" \
    --tag "$FULL_IMAGE_TAG" \
    $PUSH_FLAG \
    .

echo ""
if [ "$PUSH_FLAG" = "--push" ]; then
    echo "✅ Build and push complete: ${FULL_IMAGE_TAG}"
else
    echo "✅ Build complete: ${FULL_IMAGE_TAG}"
fi
