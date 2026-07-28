#!/bin/bash
# create-tags-local.sh
#
# Local wrapper for creating CalVer tags - extracts the CI create_tags logic
# for local development use.
#
# Usage:
#   # Auto-detect changed Dockerfiles (origin/main..HEAD)
#   ./scripts/create-tags-local.sh [--push]
#
#   # Specify commit range
#   ./scripts/create-tags-local.sh origin/main HEAD [--push]
#   ./scripts/create-tags-local.sh origin/main..HEAD [--push]
#
#   # Explicit Dockerfile paths
#   ./scripts/create-tags-local.sh dockerfiles/Dockerfile.2.14 dockerfiles/Dockerfile.2.15-dev [--push]
#
#   # Tag a specific commit (default: HEAD)
#   ./scripts/create-tags-local.sh --commit abc123 [--push]
#
# Options:
#   --push       Push tags to origin after creating them
#   --commit     Commit SHA to tag (default: HEAD)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GITHUB_SCRIPTS="$REPO_ROOT/.github/scripts"

# Default values
PUSH=false
COMMIT_SHA="HEAD"
DOCKERFILES=()
FROM_REF=""
TO_REF=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --push)
            PUSH=true
            shift
            ;;
        --commit)
            COMMIT_SHA="$2"
            shift 2
            ;;
        --help|-h)
            head -n 25 "$0" | tail -n +2 | sed 's/^# //'
            exit 0
            ;;
        *)
            # Check if it's a git range (contains ..)
            if [[ "$1" == *..* ]]; then
                # Split range
                FROM_REF="${1%%..*}"
                TO_REF="${1##*..}"
                shift
            # Check if it looks like a Dockerfile path
            elif [[ "$1" == dockerfiles/Dockerfile.* ]]; then
                DOCKERFILES+=("$1")
                shift
            # Otherwise, assume it's part of a two-arg range
            elif [[ -z "$FROM_REF" ]]; then
                FROM_REF="$1"
                shift
            elif [[ -z "$TO_REF" ]]; then
                TO_REF="$1"
                shift
            else
                echo "Error: Unknown argument: $1" >&2
                exit 1
            fi
            ;;
    esac
done

# Resolve commit SHA to full hash
COMMIT_SHA=$(git rev-parse "$COMMIT_SHA")

# Determine mode and get list of Dockerfiles
if [[ ${#DOCKERFILES[@]} -eq 0 ]]; then
    # Auto-detect mode: need to determine FROM and TO refs
    if [[ -z "$FROM_REF" ]]; then
        FROM_REF="origin/main"
    fi
    if [[ -z "$TO_REF" ]]; then
        TO_REF="HEAD"
    fi

    echo "Detecting changed Dockerfiles between $FROM_REF and $TO_REF..."
    CHANGED=$("$GITHUB_SCRIPTS/detect-changed-dockerfiles.sh" "$FROM_REF" "$TO_REF")

    if [[ -z "$CHANGED" ]]; then
        echo "No Dockerfiles changed"
        exit 0
    fi

    echo "Changed Dockerfiles:"
    echo "$CHANGED"
    echo ""

    # Convert to array
    while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            DOCKERFILES+=("$line")
        fi
    done <<< "$CHANGED"
else
    # Explicit mode
    echo "Creating tags for specified Dockerfiles:"
    printf '%s\n' "${DOCKERFILES[@]}"
    echo ""
fi

# Validate all Dockerfiles exist
for dockerfile in "${DOCKERFILES[@]}"; do
    if [[ ! -f "$REPO_ROOT/$dockerfile" ]]; then
        echo "Error: Dockerfile not found: $dockerfile" >&2
        exit 1
    fi
done

# Create tags
TAGS=()

echo "Creating CalVer tags for Dockerfiles at commit $COMMIT_SHA..."
echo ""

for dockerfile in "${DOCKERFILES[@]}"; do
    echo "Processing: $dockerfile"

    # Parse Dockerfile name
    eval $("$GITHUB_SCRIPTS/parse-dockerfile-name.sh" "$dockerfile")
    echo "  Rancher Minor: $RANCHER_MINOR"
    echo "  Build Type: $BUILD_TYPE"

    # Generate CalVer tag
    TAG=$("$GITHUB_SCRIPTS/generate-calver-tag.sh" \
        "$RANCHER_MINOR" \
        "$BUILD_TYPE" \
        "$COMMIT_SHA")
    echo "  Tag: $TAG"

    # Create tag
    "$GITHUB_SCRIPTS/create-tag.sh" "$TAG" "$COMMIT_SHA" "$dockerfile"

    TAGS+=("$TAG")
    echo ""
done

echo "Successfully created ${#TAGS[@]} tag(s):"
printf '  %s\n' "${TAGS[@]}"
echo ""

# Push if requested
if [[ "$PUSH" == true ]]; then
    echo "Pushing tags to origin..."
    for tag in "${TAGS[@]}"; do
        git push origin "$tag"
        echo "  Pushed: $tag"
    done
    echo ""
    echo "All tags pushed successfully"
else
    echo "Tags created locally. Use --push to push them to origin."
    echo "Or push manually with: git push origin ${TAGS[*]}"
fi
