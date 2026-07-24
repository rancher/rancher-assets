#!/bin/sh
set -e

# Default source and destination
SOURCE_DIR="${CHARTS_SOURCE_DIR:-/var/lib/rancher-data/local-catalogs/v2}"
DEST_DIR="${CHARTS_DEST_DIR:-/charts}"

echo "Rancher Charts Copy Script"
echo "==========================="
echo "Source: ${SOURCE_DIR}"
echo "Destination: ${DEST_DIR}"
echo ""

# Display metadata if available
if [ -n "${CHART_BRANCH}" ]; then
    echo "Chart Metadata:"
    echo "  Charts:  ${CHART_BRANCH} @ ${CHART_COMMIT:0:8}"
    echo "  Partner: ${PARTNER_BRANCH} @ ${PARTNER_COMMIT:0:8}"
    echo "  RKE2:    ${RKE2_BRANCH} @ ${RKE2_COMMIT:0:8}"
    echo ""
fi

# Verify source directory exists
if [ ! -d "${SOURCE_DIR}" ]; then
    echo "ERROR: Source directory does not exist: ${SOURCE_DIR}"
    exit 1
fi

# Create destination directory if it doesn't exist
mkdir -p "${DEST_DIR}"

# Copy charts to destination
echo "Copying charts..."
cp -r "${SOURCE_DIR}"/* "${DEST_DIR}"/

echo "✓ Charts copied successfully"
echo ""
echo "Available catalogs:"
for catalog in "${DEST_DIR}"/*; do
    [ -e "$catalog" ] || continue
    echo "  - $(basename "$catalog")"
done
