# Lock File Update Workflow

This document outlines the process for updating `../lock.yaml` and how different teams should interact with the rancher-assets repository.

## Overview

The `config.yaml` file is the **source of truth** for which upstream commits are bundled into each Rancher minor version.
The file is backed by the `lock.yaml` which helps track changes to the `config.yaml` and to the state of the target branches in that config.
When upstream repositories (charts, partner-charts, rke2-charts) get new commits, those changes must flow into rancher-assets via `lock.yaml` updates.

## Versioning: CalVer Format

This project uses **CalVer (Calendar Versioning)** with Rancher-minor aligned prefixes:

**Format:** `v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`

**Examples:**
- `v2.14-20260805T1600Z` - Stable release for Rancher 2.14
- `v2.15-20260716T1430Z-dev` - Pre-release for Rancher 2.15

**Key Points:**
- Rancher version is explicit in the tag (no arithmetic needed)
- Timestamps self-generate (no version bumping logic)
- OOB releases are trivial (just a new timestamp)
- No orphan `versions` branch needed (Git tags are source of truth)
- ISO 8601 compliant (sorts correctly)

## Current State vs. Planned Workflows

### ✅ Currently Implemented

- **Manual lock.yaml regeneration** via `make generate`
- **Manual release workflow** for Release Team (CalVer format)
- **Tag-triggered builds** via `release.yml`
- **Auto-release workflow** (`.github/workflows/auto-release.yml`) - creates tags for changed Dockerfiles
- **PR smoke testing** (`.github/workflows/pr-smoke-test.yml`) - verifies Dockerfiles can build before merge
- **Dockerfile-centric change detection** via shell scripts in `.github/scripts`

### ⚠️ Not Yet Implemented

- **Automated upstream polling** - no scheduled job to query upstream repos and create PRs

## How lock.yaml Gets Updated

### Method 1: Manual Generation (Current Standard)

**Who:** Contributors, Release Team, or Chart Teams  
**When:** When you need to pick up new upstream commits

```bash
# 1. Generate lock.yaml with latest upstream commits
make generate

# This queries all upstream repos and updates:
# - upstream-refs.prod.charts.commit
# - upstream-refs.prod.partner.commit  
# - upstream-refs.prod.rke2.commit
# - upstream-refs.dev.charts.commit
# - upstream-refs.dev.partner.commit
# - upstream-refs.dev.rke2.commit
# - copy-script-hash

# 2. Review what changed
git diff lock.yaml

# 3. Commit the changes
git add lock.yaml dockerfiles/
git commit -m "Update lock.yaml with latest upstream commits"

# 4. Create PR to main
# 5. After merge, manually trigger release or wait for auto-prerelease (when implemented)
```

### Method 2: Auto-Release Workflow (IMPLEMENTED)

**Trigger:** Push to main when `dockerfiles/**` changes  
**What it does:**
1. Detects which specific Dockerfiles changed (not just minors)
2. For each changed Dockerfile:
   - Determines if it's prod or dev variant
   - Generates CalVer timestamp tag (with `-dev` suffix for dev builds)
   - Creates annotated git tag
3. Pushes all tags atomically
4. Tags trigger build workflow (`../.github/workflows/release.yml`)

**Status:** ✅ Implemented in `.github/workflows/auto-release.yml`

**Key Improvement**: This workflow is **Dockerfile-centric**, not lock.yaml-centric. It creates individual tags for each modified Dockerfile, enabling:
- Independent release cadences for prod and dev variants
- Support for manual engineering changes to individual Dockerfiles
- Multiple releases per day for the same variant (CalVer timestamp granularity)

### Method 3: Scheduled Upstream Polling (NOT IMPLEMENTED)

**Status:** No scheduled workflow exists to automatically run `make generate` and create PRs.

**If implemented, it could:**
- Run on a schedule (e.g., daily or hourly)
- Query upstream repos for new commits
- Create automated PR if changes detected
- Chart teams would see PRs appear when they merge to upstream

## CI/CD Pipeline Architecture

The repository uses a three-tier workflow system with both automatic and manual triggers:

### Tier 0: PR Validation (pr-smoke-test.yml)

**Trigger:** Pull request with changes to `dockerfiles/**`

**Process:**
1. Detects which Dockerfiles changed in the PR
2. Builds each changed Dockerfile (without pushing)
3. Reports success/failure as PR check

**Purpose:** Catch build failures before merge

**Scripts Used:**
- `.github/scripts/detect-changed-dockerfiles.sh` - Find modified Dockerfiles
- `.github/scripts/parse-dockerfile-name.sh` - Extract version info

### Tier 1a: Auto-Release Tagging (auto-release.yml)

**Trigger:** Push to `main` branch with changes to `dockerfiles/**`

**When to use:** Automatic releases when Dockerfiles change (normal workflow)

**Process:**
1. Detects which Dockerfiles changed in the merge commit
2. For each changed Dockerfile:
   - Parses filename to get Rancher minor and build type (prod/dev)
   - Generates CalVer tag: `v{MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`
   - Creates annotated git tag
3. Pushes all tags atomically to trigger Tier 2

**Example:**
If a commit changes both `Dockerfile.2.14` and `Dockerfile.2.14-dev`:
- Creates `v2.14-20260724T1530Z` (prod tag)
- Creates `v2.14-20260724T1530Z-dev` (dev tag)
- Pushes both tags → triggers two parallel release workflows

**Scripts Used:**
- `.github/scripts/detect-changed-dockerfiles.sh` - Find modified Dockerfiles
- `.github/scripts/parse-dockerfile-name.sh` - Extract version info
- `.github/scripts/generate-calver-tag.sh` - Generate CalVer tag
- `.github/scripts/create-tag.sh` - Create annotated git tag

### Tier 1b: Manual Release (manual-release.yml)

**Trigger:** Manual workflow dispatch (GitHub UI or API)

**When to use:**
- Release specific Rancher minor(s) on demand
- Create prod or dev tags explicitly (without Dockerfile changes)
- Release from a specific commit SHA (not just HEAD)
- Release ALL active minors at once
- Emergency releases outside normal flow
- Rebuild existing Dockerfiles with new timestamps

**Inputs:**
- `rancher_minor` - Specific minor (e.g., "2.15") or empty for ALL
- `build_type` - "prod" or "dev"
- `commit_sha` - Specific commit or empty for HEAD

**Process:**
1. Determines which Rancher minors to release (one or all)
2. For each minor:
   - Generates CalVer tag for specified build type
   - Creates annotated git tag
3. Pushes all tags atomically to trigger Tier 2

**Example:**
Manually release Rancher 2.14 prod:
- Input: `rancher_minor=2.14`, `build_type=prod`
- Creates `v2.14-20260724T1530Z`
- Pushes tag → triggers release workflow

**Scripts Used:**
- `.github/scripts/generate-calver-tag.sh` - Generate CalVer tag

### Tier 2: Build and Release (release.yml)

**Trigger:** Tag push matching `v*` pattern

**Process:**
1. `parse-tag` job: Parse CalVer tag, create draft release
2. `build-and-push` job: Build multi-platform image, push to registries
3. `export-image-lists` job: Extract chart catalogs, upload as release assets
4. `publish-release` job: Generate release notes, publish release
5. `create-rancher-pr` job: Create PR to rancher/rancher (prod builds only)

**Scripts Used:**
- `make build-image` and `make push-image` - Build and push Docker image
- `scripts/export-image-lists.sh` - Generate image lists (called from Makefile)

**Dockerfile Selection:**
- Tag without `-dev` suffix → `Dockerfile.{MINOR}` (prod)
- Tag with `-dev` suffix → `Dockerfile.{MINOR}-dev` (dev)

### Complete Workflow Example

**Scenario:** Developer runs `make generate`, which updates all 4 Dockerfiles

1. **Local:** Developer commits changes, creates PR
2. **PR Check (Tier 0):**
   - `pr-smoke-test.yml` detects all 4 Dockerfiles changed
   - Builds all 4 images in parallel (no push)
   - PR check passes ✅
3. **Merge to main:**
   - Developer merges PR
4. **Auto-Release (Tier 1):**
   - `auto-release.yml` detects 4 changed Dockerfiles
   - Creates 4 CalVer tags:
     - `v2.14-20260724T1530Z`
     - `v2.14-20260724T1530Z-dev`
     - `v2.15-20260724T1530Z`
     - `v2.15-20260724T1530Z-dev`
   - Pushes all 4 tags
5. **Builds (Tier 2):**
   - 4 parallel `release.yml` workflows start
   - Each builds its specific Dockerfile
   - Each creates a GitHub release
   - Each publishes to GHCR
   - Prod builds create PRs to rancher/rancher

**Result:** 4 new releases in ~10 minutes (parallel builds)

### When to Use Manual Release vs Auto-Release

**Use Auto-Release (automatic):**
- Normal workflow after merging Dockerfile changes
- Ensures every Dockerfile change gets a release
- No manual intervention needed

**Use Manual Release (manual-release.yml):**
- **Rebuild without changes**: Want to create a new release with updated timestamp but Dockerfiles haven't changed
- **Selective release**: Only want to release specific minor(s), not all that changed
- **Tag from history**: Need to create a release tag for an older commit
- **Force prod release**: Need to create a prod tag without merging Dockerfile changes
- **Testing**: Want to test the release process manually
- **Emergency**: Need a release ASAP outside normal PR flow

**Example scenarios:**

Scenario: "I need a new v2.14 prod release but the Dockerfile didn't change"
→ Use manual-release: `rancher_minor=2.14`, `build_type=prod`

Scenario: "Dockerfiles for 2.14 and 2.15 both changed, merge to main"
→ Auto-release handles it automatically

Scenario: "I need to create a release tag for commit abc123 from last week"
→ Use manual-release: `commit_sha=abc123`

Scenario: "Only release 2.15-dev, even though 2.14 also changed"
→ Use manual-release: `rancher_minor=2.15`, `build_type=dev`

## Shell Scripts for Local Development

Developers can use the CI scripts locally:

### Build an Image Locally

```bash
# Build without pushing
make build-image TAG=v2.14-test RANCHER_MINOR=2.14
make build-image TAG=v2.14-test-dev RANCHER_MINOR=2.14 DEV=true

# Build and push (requires registry login)
make push-image TAG=v2.14-test RANCHER_MINOR=2.14
make push-image TAG=v2.14-test-dev RANCHER_MINOR=2.14 DEV=true
```

### Detect Changed Dockerfiles

```bash
# See what changed between commits
./.github/scripts/detect-changed-dockerfiles.sh HEAD~5 HEAD

# See what changed in current branch vs main
./.github/scripts/detect-changed-dockerfiles.sh origin/main HEAD
```

### Parse Dockerfile Name

```bash
# Extract version info
./.github/scripts/parse-dockerfile-name.sh dockerfiles/Dockerfile.2.14-dev
# Output:
# RANCHER_MINOR=2.14
# BUILD_TYPE=dev
```

### Generate CalVer Tag

```bash
# Generate tag for current commit
./.github/scripts/generate-calver-tag.sh 2.14 prod $(git rev-parse HEAD)
# Output: v2.14-20260724T1530Z
```

### Export Image Lists

```bash
# After building/pushing an image
./scripts/export-image-lists.sh \
  --image ghcr.io/myuser/rancher-assets:v2.14-test \
  --version v2.14-test \
  --output-dir dist/v2.14-test
# Extracts chart catalogs and generates image lists
```

## Workflows for Different Teams

### For Release Team Members

**Scenario: I want to do a release for X team**

#### Option A: Release from Current lock.yaml State

If lock.yaml is already up-to-date with the commits you want to release:

```bash
# Via Makefile:
make release-manual RELEASE=stable MINOR=2.15

# Or via script directly:
./scripts/create-manual-release.sh --release=stable --minor=2.15

# Creates CalVer tag: v2.15-20260716T1430Z (or with -dev suffix for prerelease)
```

This creates tags using the existing lock.yaml state at the specified commit.

#### Option B: Update lock.yaml First, Then Release

If you need to pick up newer upstream commits:

```bash
# 1. Pull latest main
git checkout main
git pull

# 2. Update lock.yaml with latest upstream commits
make generate

# 3. Review changes
git diff lock.yaml

# 4. Commit and push
git add lock.yaml dockerfiles/
git commit -m "Update lock.yaml for [2.15] release"
git push origin main

# 5. Trigger manual release workflow
make release-manual RELEASE=stable MINOR=2.15
```

#### Option C: Release from Specific Upstream Commits

If you need to release a specific upstream commit (not the latest):

**This is NOT currently supported.** The `make generate` command always queries the latest commit for configured branches. 

**Workaround:**
1. Manually edit lock.yaml to set desired commit SHAs (NOT RECOMMENDED)
2. OR temporarily change config.yaml branches to point to tags/commits, then regenerate

**Better solution:** This should be enhanced to support `--pin-commit` flags in the generate command.

### For Chart Team Members

**Scenario: I just merged a chart to charts/partner/rke2 repo - how do I make sure the rancher-assets image picks it up?**

#### Current Process (Manual)

```bash
# 1. Clone rancher-assets
git clone https://github.com/rancher/rancher-assets
cd rancher-assets

# 2. Create a branch
git checkout -b update-charts-$(date +%Y%m%d)

# 3. Regenerate lock.yaml to pick up your upstream commit
make generate

# Expected output shows your new commit:
# Processing Rancher minor: 2.15
#   Querying upstream repositories...
#     [prod]
#       - charts @ release-v2.15: abc12345  ← Your new commit
#       - partner @ main: def67890
#       - rke2 @ main: ghi11121

# 4. Verify your commit is in the diff
git diff lock.yaml
# Should show:
#   charts:
#     branch: release-v2.15
#-    commit: old_commit_sha
#+    commit: abc12345_your_new_commit
#     fetched-at: 2026-06-09T...

# 5. Commit and create PR
git add lock.yaml dockerfiles/
git commit -m "Update charts to include <your-chart-name> changes"
git push origin update-charts-$(date +%Y%m%d)

# 6. Create PR to main
gh pr create --title "Update charts to <commit-sha>" \
  --body "Picks up changes from rancher/charts#<PR-number>"

# 7. After merge to main:
#    - Auto-prerelease workflow (when implemented) creates CalVer tag with -dev suffix
#      Example: v2.15-20260716T1430Z-dev
#    - OR Release Team manually triggers release workflow
#    - Build workflow creates the image
```

#### Automated Process (NOT YET IMPLEMENTED)

**If scheduled polling were implemented:**

1. You merge your chart to upstream (e.g., rancher/charts)
2. Scheduled workflow in rancher-assets runs hourly
3. Detects your new commit
4. Creates automated PR: "Update lock.yaml with upstream changes"
5. PR shows which commits are new
6. PR gets reviewed and merged
7. Auto-prerelease workflow creates CalVer dev tag (e.g., v2.15-20260716T1430Z-dev)
8. Image builds automatically

**To implement this, we would need:**
- `.github/workflows/scheduled-upstream-sync.yml`
- Automated PR creation with detailed changelist
- Chart team review process

### For Contributors

**Scenario: I'm updating the base image / copy script / config**

```bash
# 1. Make your changes
vim config.yaml  # or package/copy-charts.sh

# 2. Regenerate everything
make generate

# This updates:
# - dockerfiles/Dockerfile.2.14, Dockerfile.2.15, etc. (and -dev variants)
# - lock.yaml (copy-script-hash or upstream refs)

# 3. Commit all generated files
git add config.yaml dockerfiles/ lock.yaml package/
git commit -m "Bump BCI version to 16.1"

# 4. Create PR
# 5. After merge, auto-prerelease or manual release creates images
```

## Decision Tree

```
┌─────────────────────────────────────────────────────┐
│ I need to update rancher-assets                     │
└─────────────────────┬───────────────────────────────┘
                      │
         ┌────────────┴─────────────┐
         │                          │
    ┌────▼────┐              ┌──────▼──────┐
    │ I'm     │              │ I'm Release │
    │ Chart   │              │ Team        │
    │ Team    │              │             │
    └────┬────┘              └──────┬──────┘
         │                          │
         │                          │
    ┌────▼─────────────────┐   ┌────▼──────────────────┐
    │ Did I merge to       │   │ Do I need new         │
    │ upstream charts?     │   │ upstream commits?     │
    └────┬─────────────────┘   └────┬──────────────────┘
         │                          │
    ┌────▼────┐              ┌──────▼──────┐
    │ YES:    │              │ YES:        │
    │ Create  │              │ Run         │
    │ PR to   │              │ make        │
    │ update  │              │ generate    │
    │ lock.   │              │ first       │
    │ yaml    │              │             │
    └────┬────┘              └──────┬──────┘
         │                          │
         │                          │
    ┌────▼────────────────────┐    │
    │ make generate           │    │
    │ git commit              │    │
    │ Create PR               │    │
    └────┬────────────────────┘    │
         │                          │
         │                   ┌──────▼──────┐
         └───────────────────► Trigger     │
                             │ Manual      │
                             │ Release     │
                             │ Workflow    │
                             └─────────────┘
```

## Understanding lock.yaml Structure

```yaml
chart-versions:
  "2.15":  # Rancher minor version (quoted string)
    # Versions are NOT tracked in lock.yaml - CalVer generates timestamps at release time
    
    # Upstream commit refs ARE tracked
    upstream-refs:
      prod:  # Used for stable releases (v2.15-20260805T1600Z)
        charts:
          branch: release-v2.15
          commit: abc123...  ← This gets updated by make generate
          fetched-at: 2026-06-09T19:47:10Z
        partner:
          branch: main
          commit: def456...
          fetched-at: 2026-06-09T19:47:10Z
        rke2:
          branch: main
          commit: ghi789...
          fetched-at: 2026-06-09T19:47:10Z
      
      dev:   # Used for prereleases (v2.15-20260716T1430Z-dev)
        charts:
          branch: dev-v2.15
          commit: jkl012...  ← This gets updated by make generate
          fetched-at: 2026-06-09T19:47:10Z
        # ... same structure
```

**Key Points:**
- `prod` refs are used for stable builds (CalVer without `-dev` suffix: `v2.15-20260805T1600Z`)
- `dev` refs are used for prerelease builds (CalVer with `-dev` suffix: `v2.15-20260716T1430Z-dev`)
- `commit` is the upstream SHA that will be bundled
- `fetched-at` is just a timestamp (doesn't affect builds)
- `make generate` queries the upstream repos and updates these commits
- **CalVer versions are NOT stored in lock.yaml** - they're generated on-demand with current UTC timestamp

## What Triggers a New Image Build?

```
1. lock.yaml updated (make generate)
   ↓
2. PR merged to main
   ↓
3a. Auto-prerelease workflow (when implemented)
    - Detects changed Rancher minors
    - Generates CalVer tags with -dev suffix
    - Example: v2.15-20260716T1430Z-dev
    - Tags trigger build
   
   OR
   
3b. Manual release workflow
    - Release Team triggers via Makefile/script
    - Generates CalVer tags (with or without -dev suffix)
    - Example: v2.15-20260805T1600Z (stable)
    - Tags trigger build
   ↓
4. Tag creation triggers release.yml
   ↓
5. Images built and pushed
```

## Monitoring Upstream Changes

### Current Approach (Manual)

Teams must manually run `make generate` to detect upstream changes.

### Improved Approach (Recommended)

**Implement scheduled sync workflow:**

```yaml
# .github/workflows/scheduled-upstream-sync.yml
name: Scheduled Upstream Sync

on:
  schedule:
    # Run every 4 hours during business hours (UTC)
    - cron: '0 */4 * * *'
  workflow_dispatch:  # Allow manual trigger

jobs:
  sync-upstream:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.26'
      
      - name: Generate lock.yaml
        run: make generate
      
      - name: Check for changes
        id: changes
        run: |
          git diff --exit-code lock.yaml dockerfiles/ || echo "changed=true" >> $GITHUB_OUTPUT
      
      - name: Create PR if changes detected
        if: steps.changes.outputs.changed == 'true'
        run: |
          # Parse which Rancher minors changed
          CHANGED=$(go run main.go changed-minors --from=HEAD --to=HEAD)
          
          # Create branch
          BRANCH="auto-sync-$(date +%Y%m%d-%H%M)"
          git checkout -b "$BRANCH"
          
          # Commit changes
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add lock.yaml dockerfiles/
          git commit -m "Auto-sync: Update lock.yaml with upstream changes"
          
          # Push and create PR
          git push origin "$BRANCH"
          gh pr create \
            --title "Auto-sync: Upstream changes detected" \
            --body "**Changed Rancher minors:** $CHANGED

This PR was automatically created by the scheduled upstream sync workflow.

Please review the changes and merge if appropriate." \
            --label "automated" \
            --label "upstream-sync"
```

## FAQ

### Q: Why isn't my upstream commit showing up in the image?

**A:** You need to update lock.yaml:
1. Run `make generate` to query upstream and update lock.yaml
2. Commit and merge the lock.yaml change
3. Wait for auto-prerelease or trigger manual release

### Q: How often does lock.yaml get updated automatically?

**A:** Currently it doesn't. It's manual only. Scheduled sync workflow is not implemented yet.

### Q: Can I pin a specific upstream commit?

**A:** Not currently. `make generate` always queries the latest commit for the configured branches. Enhancement needed.

### Q: I merged to dev-v2.15 branch but the dev image still has the old commit

**A:** Run `make generate` to update the dev refs in lock.yaml, then create a new dev tag (e.g., v2.15-20260716T1430Z-dev).

### Q: How do I know which upstream commits are in a released image?

```bash
# Method 1: Check lock.yaml at the tag
git show v2.15-20260805T1600Z:lock.yaml

# Method 2: Check image labels (shows commit SHAs)
docker inspect ghcr.io/rancher/rancher-assets:v2.15-20260805T1600Z | jq '.[0].Config.Labels'

# Look for:
# - io.rancher.charts.commit
# - io.rancher.partner.commit
# - io.rancher.rke2.commit
# - io.rancher.rancher-minor (shows "2.15")
# - io.rancher.build-type (shows "prod" or "dev")
```

### Q: What happens if I manually edit lock.yaml?

**A:** Don't do this unless you know what you're doing. The next `make generate` will overwrite your manual changes with the latest upstream commits. If you need to pin a specific commit, you should:
1. Temporarily modify config.yaml to point branches to a tag
2. Run `make generate`
3. Restore config.yaml
4. Commit the lock.yaml with pinned commits

## Recommended Improvements

### Priority 1: Implement Auto-Prerelease Workflow

**File:** `.github/workflows/auto-prerelease.yml`

**Purpose:** Automatically create CalVer dev tags when lock.yaml changes are merged to main

**Impact:** Eliminates manual step after merging lock.yaml updates

**Implementation:** Script already updated in `scripts/create-auto-prerelease.sh` - just needs workflow YAML

### Priority 2: Implement Scheduled Upstream Sync

**File:** `.github/workflows/scheduled-upstream-sync.yml`

**Purpose:** Automatically detect upstream changes and create PRs

**Impact:** Chart teams don't need to manually update lock.yaml

### Priority 3: Enhance Generate Command

**Add flags:**
```bash
# Pin specific commits
make generate --pin-charts=abc123 --pin-partner=def456

# Or via CLI
go run main.go generate --pin-commits='{"v1": {"prod": {"charts": "abc123"}}}'
```

**Impact:** Enables releasing specific upstream commits without editing lock.yaml

### Priority 4: Add Lock File Validation

**Check for:**
- Commits exist in upstream repos
- Branches match config.yaml
- Timestamps are recent (warn if > 7 days old)

**Impact:** Catch configuration drift and stale lock files

## Summary

**Current workflow is manual:**
1. Developer/Chart Team runs `make generate`
2. Commits lock.yaml changes
3. Merges to main
4. Release Team manually triggers release

**Documented workflow (not fully implemented):**
1. Developer/Chart Team runs `make generate`
2. Commits lock.yaml changes  
3. Merges to main
4. **Auto-prerelease workflow** auto-creates CalVer dev tags (e.g., v2.15-20260716T1430Z-dev) ← Missing
5. OR Release Team manually triggers release (creates CalVer tags)

**Ideal workflow (recommended):**
1. Chart Team merges to upstream
2. **Scheduled sync workflow** detects changes and creates PR ← Missing
3. PR reviewed and merged
4. **Auto-prerelease workflow** auto-creates CalVer dev tags ← Missing
5. Images built automatically
6. Release Team promotes dev to stable when ready (removes -dev suffix via new tag)
