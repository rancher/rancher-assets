#!/bin/bash
set -e

# Source versions helper functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/versions-helpers.sh"

usage() {
    cat <<EOF
Build release images from lock.yaml (stable or prerelease).

Usage: $0 [--with-lists]

Options:
  --with-lists  Also generate image lists after building

Example:
  $0
  $0 --with-lists
EOF
    exit 1
}

WITH_LISTS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --with-lists)
            WITH_LISTS=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

echo "⚠️  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "⚠️  LOCAL DEBUG BUILD - NOT FOR PRODUCTION RELEASES"
echo "⚠️  Use GitHub Actions workflows for real releases"
echo "⚠️  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Building release versions from lock.yaml..."
echo ""

CHART_MAJORS=$(yq eval '.chart-versions | keys | .[]' lock.yaml)
if [ -z "$CHART_MAJORS" ]; then
    echo "❌ Error: No chart versions found in lock.yaml"
    exit 1
fi

BUILT_COUNT=0
VERSIONS_BUILT=()

for major in $CHART_MAJORS; do
    # Try to get stable version from versions branch
    VERSION=$(versions_get "$major" "stable" 2>/dev/null || echo "")
    VERSION_TYPE="stable"

    # Fall back to prerelease if no stable
    if [ -z "$VERSION" ]; then
        VERSION=$(versions_get "$major" "prerelease" 2>/dev/null || echo "")
        VERSION_TYPE="prerelease"
    fi

    if [ -z "$VERSION" ]; then
        echo "⏭️  Skipping $major (no stable or prerelease version in versions branch)"
        continue
    fi

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Building $major at $VERSION ($VERSION_TYPE)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    make build CHART_MAJOR="$major" VERSION="$VERSION"
    if [ $? -ne 0 ]; then
        echo "❌ Build failed for $major"
        exit 1
    fi

    BUILT_COUNT=$((BUILT_COUNT + 1))
    VERSIONS_BUILT+=("$major:$VERSION")
    echo ""
done

if [ $BUILT_COUNT -eq 0 ]; then
    echo "⚠️  No releases built - no chart versions have latest-stable or latest-prerelease set"
    exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Built $BUILT_COUNT release(s) - LOCAL DEBUG ONLY"
echo "⚠️  Remember: Use GitHub Actions for production releases"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Generate image lists if requested
if [ "$WITH_LISTS" = "true" ]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Generating image lists for built releases..."
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    EXPORTED_COUNT=0

    for version_entry in "${VERSIONS_BUILT[@]}"; do
        major="${version_entry%%:*}"
        version="${version_entry##*:}"

        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "Exporting image lists for $major at $version"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

        make export-images CHART_MAJOR="$major" VERSION="$version" LOCAL=true
        if [ $? -ne 0 ]; then
            echo "❌ Image list export failed for $major"
            exit 1
        fi

        EXPORTED_COUNT=$((EXPORTED_COUNT + 1))
        echo ""
    done

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "✅ Generated image lists for $EXPORTED_COUNT release(s)"
    echo "📁 Output: dist/<version>/"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi
