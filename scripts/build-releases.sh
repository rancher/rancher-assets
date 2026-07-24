#!/bin/bash
set -e

# Local debug script to build images with CalVer dev versions
# Generates CalVer timestamps for all Rancher minors and builds them locally

usage() {
    cat <<EOF
Build all Rancher minors with auto-generated CalVer dev versions.

Usage: $0 [--with-lists]

Options:
  --with-lists  Also generate image lists after building

Example:
  $0
  $0 --with-lists

Note: This is LOCAL DEBUG ONLY. Use GitHub Actions workflows for production releases.
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
echo "Building all Rancher minors with CalVer dev versions..."
echo ""

RANCHER_MINORS=$(yq eval '.chart-versions | keys | .[]' config.yaml)
if [ -z "$RANCHER_MINORS" ]; then
    echo "❌ Error: No Rancher minors found in config.yaml"
    exit 1
fi

BUILT_COUNT=0
VERSIONS_BUILT=()

for minor in $RANCHER_MINORS; do
    # Generate CalVer dev version for this minor
    VERSION=$(go run main.go calver-dev-version --minor="$minor")

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Building Rancher $minor at $VERSION (dev)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    make build RANCHER_MINOR="$minor" VERSION="$VERSION"
    if [ $? -ne 0 ]; then
        echo "❌ Build failed for Rancher $minor"
        exit 1
    fi

    BUILT_COUNT=$((BUILT_COUNT + 1))
    VERSIONS_BUILT+=("$minor:$VERSION")
    echo ""
done

if [ $BUILT_COUNT -eq 0 ]; then
    echo "⚠️  No releases built"
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
        minor="${version_entry%%:*}"
        version="${version_entry##*:}"

        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "Exporting image lists for Rancher $minor at $version"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

        make export-images RANCHER_MINOR="$minor" VERSION="$version" LOCAL=true
        if [ $? -ne 0 ]; then
            echo "❌ Image list export failed for Rancher $minor"
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
