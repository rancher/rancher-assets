#!/bin/bash
set -e

usage() {
    cat <<EOF
Get build variables for a Rancher minor version.

Reads config.yaml and lock.yaml to extract all the branches, commits, and
configuration needed to build a chart image. Outputs variables in a format
that can be sourced by the shell.

Usage: $0 --minor <minor> --version <version> [--format <format>]

Required:
  --minor       Rancher minor version (e.g., 2.15)
  --version     Image version tag (e.g., v2.15-20260716T1430Z, v2.15-20260716T1430Z-dev)

Optional:
  --format      Output format: env (default), json, shell

Output formats:
  env    - KEY=value (one per line, can be sourced)
  json   - JSON object
  shell  - Shell variable assignments (eval-able)

Example:
  $0 --minor 2.15 --version v2.15-20260716T1430Z-dev
  eval \$($0 --minor 2.15 --version v2.15-20260716T1430Z --format shell)
EOF
    exit 1
}

RANCHER_MINOR=""
VERSION=""
FORMAT="env"

while [[ $# -gt 0 ]]; do
    case $1 in
        --minor)
            RANCHER_MINOR="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --format)
            FORMAT="$2"
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

if [ -z "$RANCHER_MINOR" ] || [ -z "$VERSION" ]; then
    echo "❌ Error: --minor and --version are required" >&2
    exit 1
fi

# Detect build type from version tag (CalVer with -dev suffix = dev, without = prod)
if echo "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+-[0-9]{8}T[0-9]{4}Z-dev$'; then
    BUILD_TYPE=dev
elif echo "$VERSION" | grep -qE '^v[0-9]+\.[0-9]+-[0-9]{8}T[0-9]{4}Z$'; then
    BUILD_TYPE=prod
else
    echo "❌ Error: Invalid CalVer version format: $VERSION" >&2
    echo "Expected: v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]" >&2
    exit 1
fi

# Read configuration from config.yaml and lock.yaml
RANCHER_BRANCH=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".rancher-branch" config.yaml 2>/dev/null)

CHART_BRANCH=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".upstream-refs.$BUILD_TYPE.charts.branch" lock.yaml 2>/dev/null)
PARTNER_BRANCH=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".upstream-refs.$BUILD_TYPE.partner.branch" lock.yaml 2>/dev/null)
RKE2_BRANCH=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".upstream-refs.$BUILD_TYPE.rke2.branch" lock.yaml 2>/dev/null)

CHART_COMMIT=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".upstream-refs.$BUILD_TYPE.charts.commit" lock.yaml 2>/dev/null)
PARTNER_COMMIT=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".upstream-refs.$BUILD_TYPE.partner.commit" lock.yaml 2>/dev/null)
RKE2_COMMIT=$(yq eval ".chart-versions.\"$RANCHER_MINOR\".upstream-refs.$BUILD_TYPE.rke2.commit" lock.yaml 2>/dev/null)

# Validate configuration
if [ "$CHART_BRANCH" = "null" ] || [ -z "$CHART_BRANCH" ]; then
    echo "❌ Error: $BUILD_TYPE refs not found in lock.yaml for $RANCHER_MINOR" >&2
    echo "Run 'make generate' first" >&2
    exit 1
fi

if [ "$CHART_COMMIT" = "null" ] || [ -z "$CHART_COMMIT" ]; then
    echo "❌ Error: $BUILD_TYPE commits not found in lock.yaml for $RANCHER_MINOR" >&2
    echo "Run 'make generate' first" >&2
    exit 1
fi

# Output in requested format
case $FORMAT in
    env)
        cat <<EOF
BUILD_TYPE=$BUILD_TYPE
RANCHER_BRANCH=$RANCHER_BRANCH
CHART_BRANCH=$CHART_BRANCH
PARTNER_BRANCH=$PARTNER_BRANCH
RKE2_BRANCH=$RKE2_BRANCH
CHART_COMMIT=$CHART_COMMIT
PARTNER_COMMIT=$PARTNER_COMMIT
RKE2_COMMIT=$RKE2_COMMIT
EOF
        ;;
    shell)
        cat <<EOF
BUILD_TYPE="$BUILD_TYPE"
RANCHER_BRANCH="$RANCHER_BRANCH"
CHART_BRANCH="$CHART_BRANCH"
PARTNER_BRANCH="$PARTNER_BRANCH"
RKE2_BRANCH="$RKE2_BRANCH"
CHART_COMMIT="$CHART_COMMIT"
PARTNER_COMMIT="$PARTNER_COMMIT"
RKE2_COMMIT="$RKE2_COMMIT"
EOF
        ;;
    json)
        cat <<EOF
{
  "buildType": "$BUILD_TYPE",
  "rancherBranch": "$RANCHER_BRANCH",
  "chartBranch": "$CHART_BRANCH",
  "partnerBranch": "$PARTNER_BRANCH",
  "rke2Branch": "$RKE2_BRANCH",
  "chartCommit": "$CHART_COMMIT",
  "partnerCommit": "$PARTNER_COMMIT",
  "rke2Commit": "$RKE2_COMMIT"
}
EOF
        ;;
    *)
        echo "❌ Error: Invalid format: $FORMAT" >&2
        echo "Valid formats: env, json, shell" >&2
        exit 1
        ;;
esac
