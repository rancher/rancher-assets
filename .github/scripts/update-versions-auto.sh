#!/bin/bash
set -e

COMMIT_SHA="$1"
CHANGED_MAJORS="$2"

git commit -m "Auto-update prerelease versions

Triggered by: push to main ($COMMIT_SHA)
Changed majors: $CHANGED_MAJORS"

echo "Committed version updates"
