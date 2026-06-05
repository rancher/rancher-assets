#!/bin/bash
# Helper functions for managing versions.yaml on the orphan 'versions' branch

VERSIONS_BRANCH="versions"
VERSIONS_WORKTREE="/tmp/rancher-assets-versions-$$"
VERSIONS_FILE="versions.yaml"

# versions_checkout: Check out versions branch as a worktree
# Usage: versions_checkout
versions_checkout() {
    if [ -d "$VERSIONS_WORKTREE" ]; then
        echo "⚠️  Versions worktree already exists at $VERSIONS_WORKTREE" >&2
        return 1
    fi

    # Check if versions branch exists
    if ! git rev-parse --verify "$VERSIONS_BRANCH" >/dev/null 2>&1; then
        echo "❌ Error: '$VERSIONS_BRANCH' branch does not exist" >&2
        return 1
    fi

    # Create worktree
    git worktree add "$VERSIONS_WORKTREE" "$VERSIONS_BRANCH" >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo "❌ Error: Failed to create versions worktree" >&2
        return 1
    fi

    echo "$VERSIONS_WORKTREE"
}

# versions_cleanup: Clean up the versions worktree
# Usage: versions_cleanup
versions_cleanup() {
    if [ -d "$VERSIONS_WORKTREE" ]; then
        git worktree remove "$VERSIONS_WORKTREE" --force >/dev/null 2>&1
    fi
}

# versions_get: Get a version from versions.yaml
# Usage: versions_get <major> <type>
#   major: v0, v1, etc.
#   type: stable or prerelease
# Returns: version tag or empty string
versions_get() {
    local major="$1"
    local type="$2"

    if [ -z "$major" ] || [ -z "$type" ]; then
        echo "Usage: versions_get <major> <type>" >&2
        return 1
    fi

    local worktree
    worktree=$(versions_checkout)
    if [ $? -ne 0 ]; then
        return 1
    fi

    local version
    version=$(yq eval ".\"$major\".$type.tag" "$VERSIONS_WORKTREE/$VERSIONS_FILE" 2>/dev/null)

    versions_cleanup

    if [ "$version" = "null" ] || [ -z "$version" ]; then
        return 1
    fi

    echo "$version"
}

# versions_set: Set a version in versions.yaml
# Usage: versions_set <major> <type> <tag>
#   major: v0, v1, etc.
#   type: stable or prerelease
#   tag: version tag (e.g., v1.0.0)
versions_set() {
    local major="$1"
    local type="$2"
    local tag="$3"

    if [ -z "$major" ] || [ -z "$type" ] || [ -z "$tag" ]; then
        echo "Usage: versions_set <major> <type> <tag>" >&2
        return 1
    fi

    local worktree
    worktree=$(versions_checkout)
    if [ $? -ne 0 ]; then
        return 1
    fi

    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Update the version
    yq eval -i ".\"$major\".$type.tag = \"$tag\"" "$VERSIONS_WORKTREE/$VERSIONS_FILE"
    yq eval -i ".\"$major\".$type.updated-at = \"$timestamp\"" "$VERSIONS_WORKTREE/$VERSIONS_FILE"

    # Commit and push
    cd "$VERSIONS_WORKTREE"
    git add "$VERSIONS_FILE"
    git commit -m "chore: update $major $type to $tag" >/dev/null 2>&1
    git push origin "$VERSIONS_BRANCH" >/dev/null 2>&1
    local result=$?
    cd - >/dev/null

    versions_cleanup

    if [ $result -ne 0 ]; then
        echo "❌ Error: Failed to update versions branch" >&2
        return 1
    fi

    echo "✅ Updated $major $type to $tag"
}

# versions_list: List all versions
# Usage: versions_list [--major <major>]
versions_list() {
    local filter_major=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --major)
                filter_major="$2"
                shift 2
                ;;
            *)
                echo "Unknown option: $1" >&2
                return 1
                ;;
        esac
    done

    local worktree
    worktree=$(versions_checkout)
    if [ $? -ne 0 ]; then
        return 1
    fi

    if [ -n "$filter_major" ]; then
        # Show specific major
        echo "Major: $filter_major"
        echo "  Stable: $(yq eval ".\"$filter_major\".stable.tag" "$VERSIONS_WORKTREE/$VERSIONS_FILE")"
        echo "  Prerelease: $(yq eval ".\"$filter_major\".prerelease.tag" "$VERSIONS_WORKTREE/$VERSIONS_FILE")"
    else
        # Show all majors
        cat "$VERSIONS_WORKTREE/$VERSIONS_FILE"
    fi

    versions_cleanup
}

# If sourced, make functions available
# If executed directly, run as CLI
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    # Direct execution - provide CLI
    case "${1:-help}" in
        get)
            versions_get "$2" "$3"
            ;;
        set)
            versions_set "$2" "$3" "$4"
            ;;
        list)
            shift
            versions_list "$@"
            ;;
        help|--help|-h)
            cat <<EOF
Manage versions.yaml on the orphan 'versions' branch.

Usage:
  $0 get <major> <type>          Get version
  $0 set <major> <type> <tag>    Set version
  $0 list [--major <major>]       List versions

Examples:
  $0 get v1 prerelease
  $0 set v1 prerelease v1.0.0-rc.1
  $0 list
  $0 list --major v1

Functions (when sourced):
  versions_get <major> <type>
  versions_set <major> <type> <tag>
  versions_list [--major <major>]
  versions_checkout
  versions_cleanup
EOF
            ;;
        *)
            echo "Unknown command: $1" >&2
            echo "Run '$0 help' for usage" >&2
            exit 1
            ;;
    esac
fi
