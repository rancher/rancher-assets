#!/bin/sh
# parse-dockerfile-name.sh
#
# Extracts version information from a Dockerfile path.
# Outputs environment variable assignments for use in GitHub Actions.
#
# Usage: parse-dockerfile-name.sh <dockerfile_path>
# Output: RANCHER_MINOR={version}
#         BUILD_TYPE={prod|dev}
#
# Examples:
#   Input:  dockerfiles/Dockerfile.2.14
#   Output: RANCHER_MINOR=2.14
#           BUILD_TYPE=prod
#
#   Input:  dockerfiles/Dockerfile.2.15-dev
#   Output: RANCHER_MINOR=2.15
#           BUILD_TYPE=dev
#
# In GitHub Actions, use with: eval $(./script.sh path)

set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <dockerfile_path>" >&2
    echo "Example: $0 dockerfiles/Dockerfile.2.14-dev" >&2
    exit 1
fi

DOCKERFILE_PATH="$1"

# Extract filename from path
FILENAME=$(basename "$DOCKERFILE_PATH")

# Remove "Dockerfile." prefix
# Expected format: Dockerfile.X.XX or Dockerfile.X.XX-dev
VERSION_PART=$(echo "$FILENAME" | sed 's/^Dockerfile\.//')

# Check if it has -dev suffix
if echo "$VERSION_PART" | grep -q -- '-dev$'; then
    # Dev variant
    BUILD_TYPE="dev"
    # Remove -dev suffix to get version
    RANCHER_MINOR=$(echo "$VERSION_PART" | sed 's/-dev$//')
else
    # Prod variant
    BUILD_TYPE="prod"
    RANCHER_MINOR="$VERSION_PART"
fi

# Validate that we got a version number (format: X.XX)
if ! echo "$RANCHER_MINOR" | grep -qE '^[0-9]+\.[0-9]+$'; then
    echo "Error: Invalid Dockerfile name format: $FILENAME" >&2
    echo "Expected: Dockerfile.X.XX or Dockerfile.X.XX-dev" >&2
    exit 1
fi

# Output as environment variable assignments
echo "RANCHER_MINOR=$RANCHER_MINOR"
echo "BUILD_TYPE=$BUILD_TYPE"
