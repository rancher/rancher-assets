#!/bin/bash
set -e

VERSION="$1"
MAJOR="$2"
IMAGE="$3"

cat <<EOF
## Rancher Assets Update

**Version**: \`${VERSION}\`
**Chart Major**: \`${MAJOR}\`
**Image**: \`${IMAGE}\`

### Changes
This updates the rancher-assets image to ${VERSION}.

### Updates Required
- [ ] Update \`build.yaml\`: \`defaultChartsImage: ${IMAGE}\`
- [ ] Run \`go generate\` to update chart values

🤖 Automated PR from rancher-assets release workflow
EOF
