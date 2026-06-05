#!/bin/bash
set -e

VERSION="$1"
MAJOR="$2"
BUILD_TYPE="$3"
IMAGE="$4"
TARGET_BRANCH="$5"
CHART_BRANCH="$6"
CHART_COMMIT="$7"
PARTNER_BRANCH="$8"
PARTNER_COMMIT="$9"
RKE2_BRANCH="${10}"
RKE2_COMMIT="${11}"

cat <<EOF
## Rancher Assets ${VERSION}

**Chart Major**: ${MAJOR}
**Target Rancher Branch**: ${TARGET_BRANCH}
**Build Type**: ${BUILD_TYPE}

### Image
\`\`\`
${IMAGE}
\`\`\`

### Upstream Sources
| Repository | Branch | Commit |
|------------|--------|--------|
| rancher/charts | ${CHART_BRANCH} | \`${CHART_COMMIT:0:8}\` |
| rancher/partner-charts | ${PARTNER_BRANCH} | \`${PARTNER_COMMIT:0:8}\` |
| rancher/rke2-charts | ${RKE2_BRANCH} | \`${RKE2_COMMIT:0:8}\` |

### Multi-arch Image
\`${IMAGE}\` (linux/amd64, linux/arm64)
EOF
