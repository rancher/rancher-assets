#!/bin/bash
set -e

usage() {
    cat <<EOF
Generate release notes for rancher-assets release.

Usage: $0 <version> <major> <build-type> [--draft] [--tag-message <message>]

Required:
  version       Version tag (e.g., v1.0.0)
  major         Chart major (e.g., v1)
  build-type    Build type (prod or dev)

Optional:
  --draft              Include draft warning message
  --tag-message <msg>  Include tag message in release notes

Example:
  $0 v1.0.0 v1 prod
  $0 v1.0.0-rc.1 v1 dev --draft
EOF
    exit 1
}

VERSION=""
MAJOR=""
BUILD_TYPE=""
DRAFT=false
TAG_MESSAGE=""

# Parse positional arguments
if [ $# -lt 3 ]; then
    usage
fi

VERSION="$1"
MAJOR="$2"
BUILD_TYPE="$3"
shift 3

# Parse optional flags
while [[ $# -gt 0 ]]; do
    case $1 in
        --draft)
            DRAFT=true
            shift
            ;;
        --tag-message)
            TAG_MESSAGE="$2"
            shift 2
            ;;
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage
            ;;
    esac
done

# Get build vars to populate upstream info
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
eval $(${SCRIPT_DIR}/../../scripts/get-build-vars.sh --major "$MAJOR" --version "$VERSION" --format shell 2>/dev/null || echo "")

# Start building release notes
cat <<EOF
## Rancher Assets ${VERSION}

**Chart Major**: ${MAJOR}
**Build Type**: ${BUILD_TYPE}
**Target Branch**: ${RANCHER_BRANCH}
EOF

# Add tag message if provided
if [ -n "$TAG_MESSAGE" ]; then
    cat <<EOF

---

${TAG_MESSAGE}

---
EOF
fi

# Add draft warning if this is a draft
if [ "$DRAFT" = "true" ]; then
    cat <<EOF

> [!WARNING]
> **This release is being built.** It will be published automatically when all jobs complete successfully.

EOF
fi

# Add upstream sources table if we have the data
if [ -n "$CHART_BRANCH" ]; then
    cat <<EOF

### Upstream Sources
| Repository | Branch | Commit |
|------------|--------|--------|
| rancher/charts | ${CHART_BRANCH} | \`${CHART_COMMIT:0:8}\` |
| rancher/partner-charts | ${PARTNER_BRANCH} | \`${PARTNER_COMMIT:0:8}\` |
| rancher/rke2-charts | ${RKE2_BRANCH} | \`${RKE2_COMMIT:0:8}\` |
EOF
fi

# Add image lists note if not draft
if [ "$DRAFT" = "false" ]; then
    cat <<EOF

### Air-Gapped Deployment

Image lists and helper scripts for air-gapped deployments are available as release assets below:
- \`rancher-charts-images.txt\` - Linux container images
- \`rancher-charts-windows-images.txt\` - Windows container images
- Helper scripts for save/load/mirror operations
EOF
fi
