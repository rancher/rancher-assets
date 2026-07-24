#!/bin/bash
set -e

usage() {
    cat <<EOF
Extract chart catalogs from a Docker image and generate image lists.

Usage: $0 --image <image> --version <version> --output-dir <dir> [--local]

Options:
  --image       Full image name (e.g., ghcr.io/rancher/rancher-assets:v1.0.0)
  --version     Version tag (e.g., v1.0.0)
  --output-dir  Output directory for image lists
  --local       Skip pulling image (use local image)

Example:
  $0 --image ghcr.io/rancher/rancher-assets:v1.0.0 --version v1.0.0 --output-dir dist/v1.0.0
EOF
    exit 1
}

IMAGE=""
VERSION=""
OUTPUT_DIR=""
LOCAL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --image)
            IMAGE="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --local)
            LOCAL=true
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

if [ -z "$IMAGE" ] || [ -z "$VERSION" ] || [ -z "$OUTPUT_DIR" ]; then
    usage
fi

echo "Generating image lists for $VERSION..."

# Pull image if not local
if [ "$LOCAL" != "true" ]; then
    echo "Pulling image from registry..."
    docker pull "$IMAGE"
else
    echo "Using local image (skipping pull)..."
fi

# Extract chart catalogs
TEMP_DIR=${TEMP_DIR:-"/tmp/rancher-assets-charts-$VERSION"}
echo "Extracting chart catalogs to $TEMP_DIR..."
CONTAINER_ID=$(docker create "$IMAGE")
rm -rf "$TEMP_DIR"
mkdir -p "$TEMP_DIR"
docker cp "$CONTAINER_ID:/var/lib/rancher-data/local-catalogs/v2" "$TEMP_DIR/"
docker rm "$CONTAINER_ID" >/dev/null

# The extracted catalogs are bare git repos - need to checkout the working tree
echo "Checking out repo contents..."
for catalog_dir in "$TEMP_DIR/v2"/*; do
    if [ -d "$catalog_dir/.git" ]; then
        echo "  Checking out $(basename "$catalog_dir")..."
        (cd "$catalog_dir" && git config --local core.bare false && git checkout -- . 2>/dev/null || true)
    fi
done

# Run image list generator
echo "Scanning charts for image references..."
mkdir -p "$OUTPUT_DIR"
go run main.go export-images \
    --charts-path "$TEMP_DIR/v2" \
    --version "$VERSION" \
    --output-dir "$OUTPUT_DIR"

echo ""
echo "✅ Image lists generated in $OUTPUT_DIR/"
echo ""
ls -lh "$OUTPUT_DIR/"
