#!/bin/bash
set -e

usage() {
    cat <<EOF
Update Go dependencies and vendor them.

Usage: $0

This script will:
  1. Update all Go dependencies (go get -u)
  2. Tidy go.mod and go.sum (go mod tidy)
  3. Vendor dependencies (go mod vendor)

Example:
  $0
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

echo "Updating Go dependencies..."
go get -u ./...

echo "Tidying go.mod..."
go mod tidy

echo "Vendoring dependencies..."
go mod vendor

echo ""
echo "✅ Dependencies updated and vendored"
echo ""
echo "Review changes with: git diff go.mod go.sum vendor/"
echo "Commit with: git add go.mod go.sum vendor/ && git commit -m 'chore: update Go dependencies'"
