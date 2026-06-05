#!/bin/bash
set -e

RELEASE_TYPE="$1"
BUMP_TYPE="$2"
CHART_MAJORS="$3"
COMMIT_SHA="$4"

git commit -m "Update versions after manual release

Release Type: $RELEASE_TYPE
Bump Type: $BUMP_TYPE
Chart Majors: $CHART_MAJORS
Commit: $COMMIT_SHA"

echo "Committed version updates"
