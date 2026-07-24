# Versioning

This document defines how rancher-assets handles versioning for chart images.

## Rancher-Minor Aligned CalVer

This repository uses **CalVer (Calendar Versioning)** with **Rancher minor version prefixes**. Each version explicitly shows which Rancher minor it's compatible with, using ISO 8601 timestamps to create unique, sortable versions.

### Rancher Minor Alignment

| Rancher Minor | Version Prefix | Charts Branch (prod) | Charts Branch (dev) | Status |
|---------------|----------------|----------------------|---------------------|---------|
| 2.14.x        | v2.14-         | release-v2.14        | dev-v2.14           | Active  |
| 2.15.x        | v2.15-         | release-v2.15        | dev-v2.15           | Active  |
| 2.16.x        | v2.16-         | release-v2.16        | dev-v2.16           | Active  |

**Note:** This table will grow as new Rancher minor versions are released. Each new Rancher minor (2.16, 2.17, etc.) gets a corresponding version prefix (v2.16-, v2.17-, etc.).

## Version Format

Chart images use CalVer with ISO 8601 timestamps:

### Production Releases (Stable)

Production releases use UTC timestamps without the dev suffix:

```
v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z
```

**Examples:**
- `v2.14-20260716T1430Z` - Rancher 2.14.3 GA release (July 16, 2026 at 14:30 UTC)
- `v2.14-20260722T0915Z` - OOB release for Longhorn CVE (July 22, 2026 at 09:15 UTC)
- `v2.15-20260805T1600Z` - Rancher 2.15.0 GA release (August 5, 2026 at 16:00 UTC)

### Pre-Releases

Pre-releases use the same timestamp format with a `-dev` suffix:

```
v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z-dev
```

**Examples:**
- `v2.14-20260819T1130Z-dev` - First RC for Rancher 2.14.4 (August 19, 2026 at 11:30 UTC)
- `v2.14-20260819T1530Z-dev` - Second RC same day (time differentiates: 15:30 UTC)
- `v2.15-20261212T1030Z-dev` - Alpha/RC for Rancher 2.15.0 (December 12, 2026 at 10:30 UTC)

Pre-releases are:
- **Auto-generated** when changes merge to main
- **Manually triggered** by Release Team for validation
- Built from appropriate **dev** or **prod** branches in lock.yaml
- Intended for testing before promoting to stable

### Version Properties

**Why CalVer works for rancher-assets:**

✅ **Rancher version explicit** - `v2.14-*` shows compatibility at a glance  
✅ **OOB releases trivial** - Just use a new timestamp, no version arithmetic  
✅ **No SemVer constraints** - Consumed as image tags (freeform strings)  
✅ **Multiple releases per day** - Time component differentiates (T1130Z vs T1530Z)  
✅ **Lexicographic sort** - ISO 8601 sorts correctly as strings  
✅ **No mapping tables** - Version directly indicates Rancher minor  
✅ **Self-generating** - Timestamps don't require sequence counters  

**Trade-offs:**

⚠️ **Not SemVer compliant** - Irrelevant (no Helm chart, no parsers)  
⚠️ **Cannot target specific Rancher patch** - All `v2.14-*` work with any 2.14.x  
⚠️ **All timestamps in UTC** - Z suffix indicates Zulu time (UTC+0)

## Version Tracking: Git Tags

Versions are tracked via **Git tags** pointing to commits on `main`. No separate branch needed since timestamps are self-generating.

### Why Git Tags Only?

✅ **Self-generating** - Timestamps created at release time, no sequence tracking  
✅ **No noise on main** - Tags don't clutter code history  
✅ **Simple audit trail** - Git tags show release history  
✅ **No workflow loops** - Tag creation doesn't trigger rebuilds  

### Querying Versions

```bash
# All stable releases for Rancher 2.14
git tag -l "v2.14-*" --sort=-version:refname | grep -v -- '-dev$'

# All pre-releases for Rancher 2.14
git tag -l "v2.14-*" --sort=-version:refname | grep -- '-dev$'

# Latest stable for Rancher 2.15
git tag -l "v2.15-*" --sort=-version:refname | grep -v -- '-dev$' | head -1
# v2.15-20260805T1600Z

# When was a specific version released?
git log -1 --format='%ai' v2.14-20260716T1430Z
# 2026-07-16 14:30:00 +0000

# What's in a release?
git show v2.14-20260716T1430Z:lock.yaml
```

## Lock File: Upstream Commit Tracking Only

The `lock.yaml` file on `main` branch tracks **upstream repository commits only**:

```yaml
chart-versions:
  v0:
    upstream-refs:
      prod:  # Production upstream refs (release-v2.14 branches)
        charts: { branch: release-v2.14, commit: f408a794..., fetched-at: ... }
        partner: { branch: main, commit: 13b90384..., fetched-at: ... }
        rke2: { branch: main, commit: d0865878..., fetched-at: ... }
      dev:   # Development upstream refs (dev-v2.14 branches)
        charts: { branch: dev-v2.14, commit: a7b9d818..., fetched-at: ... }
        partner: { branch: main, commit: 13b90384..., fetched-at: ... }
        rke2: { branch: main, commit: d0865878..., fetched-at: ... }
```

**The lock file does NOT track release versions.**

## Build Types

The build system automatically detects build type from the version tag:

| Version Format | Build Type | Upstream Refs | Usage |
|---------------|------------|---------------|-------|
| `v{RANCHER_MINOR}-{TIMESTAMP}Z` (no -dev) | `prod` | `lock.yaml` → `upstream-refs.prod` | Production releases |
| `v{RANCHER_MINOR}-{TIMESTAMP}Z-dev` | `dev` | `lock.yaml` → `upstream-refs.dev` | Pre-releases |

This detection happens in the workflow and determines which upstream branch/commit refs are used from lock.yaml.

## Release Workflows

### 1. Automatic Pre-Releases (on merge to main)

**Workflow:** `.github/workflows/auto-prerelease.yml`

**Trigger:** Push to `main` branch when `lock.yaml` changes

**Why lock.yaml?** It's the source of truth for all build inputs:
- Upstream commit refs (what charts are bundled)
- Package script hash (how charts are copied)
- Generated by `make generate` on config/package/upstream changes

**What it does:**
1. Diffs lock.yaml to detect which Rancher minors have upstream ref changes
2. Skips if only timestamps changed (no meaningful changes)
3. Generates CalVer timestamps (UTC)
4. Creates pre-release git tags with `-dev` suffix
5. Tag triggers build workflow

**Example:**
```
PR: Bump BCI version
  ↓
Run: make generate (updates lock.yaml upstream refs)
  ↓
Merge to main (commit abc123) at 2026-07-16 14:30:00 UTC
  ↓
Auto-prerelease workflow detects:
  - lock.yaml changed
  - Diffs upstream refs: 2.14 and 2.15 have new commits
  ↓
Generates timestamps:
  Current UTC: 2026-07-16T1430Z
  ↓
Creates tags on commit abc123:
  - v2.14-20260716T1430Z-dev
  - v2.15-20260716T1430Z-dev
  ↓
Build workflow builds both images from abc123
```

### 2. Manual Releases (Release Team Control)

**Workflow:** `.github/workflows/manual-release.yml`

**Trigger:** Manual `workflow_dispatch`

**Inputs:**
- `commit_sha` - Commit to release (default: HEAD of main)
- `rancher_minor` - Which Rancher minor (e.g., 2.14, 2.15; empty = ALL)
- `release_type` - `prerelease` (default) or `stable`

**Use Cases:**

#### Scenario A: Rancher 2.15.0 GA release
```
Release Team triggers:
  commit_sha: [empty] (uses HEAD)
  rancher_minor: 2.15
  release_type: stable

Result:
  v2.15-20260805T1600Z (stable)
  Tag created on HEAD of main at 16:00 UTC
```

#### Scenario B: OOB release for Rancher 2.14.3
```
Release Team triggers:
  commit_sha: def456
  rancher_minor: 2.14
  release_type: stable

Result:
  v2.14-20260722T0915Z (stable)
  Tag created on commit def456 at 09:15 UTC
```

#### Scenario C: BCI security fix affects all Rancher minors
```
Release Team triggers:
  commit_sha: abc123
  rancher_minor: [empty] ← ALL
  release_type: prerelease

Result:
  v2.14-20260716T1430Z-dev
  v2.15-20260716T1430Z-dev
  All tags on same commit abc123 at 14:30 UTC
```

#### Scenario D: RC for upcoming Rancher 2.14.4
```
Release Team triggers:
  commit_sha: [empty]
  rancher_minor: 2.14
  release_type: prerelease

Result:
  v2.14-20260819T1130Z-dev
  Ready for validation before stable
```

### 3. Build and Release (on tag)

**Workflow:** `.github/workflows/build-release.yml`

**Trigger:** Tag matching `v*` pattern

**What it does:**
1. Parses tag to determine version and build type
2. Builds multi-arch images (amd64, arm64)
3. Pushes to Docker Hub
4. Creates GitHub Release with metadata
5. Creates PR to `rancher/rancher` (stable only)

**Tag triggers build immediately:**
```
Tag created: v2.15-20260805T1600Z
  ↓
build-release.yml triggers
  ↓
Reads lock.yaml from tagged commit
  ↓
Builds using prod refs (no -dev suffix = stable)
  ↓
Pushes multi-arch image:
  - ghcr.io/rancher/rancher-assets:v2.15-20260805T1600Z (linux/amd64, linux/arm64)
  ↓
Creates GitHub Release
  ↓
Creates PR to rancher/rancher (if stable)
```

## Version Lifecycle

### Development Flow

```bash
# 1. Make changes (e.g., bump BCI version)
make generate
git commit -m "Bump BCI to fix CVE"

# 2. Create PR, merge to main
# (PR reviewed, CI validates)

# 3. Auto-prerelease workflow triggers at 14:30 UTC
# - Detects changed dockerfiles
# - Generates CalVer timestamps
# - Creates tags with -dev suffix
# - Builds images

# 4. Images available for testing
# ghcr.io/rancher/rancher-assets:v2.14-20260716T1430Z-dev
# ghcr.io/rancher/rancher-assets:v2.15-20260716T1430Z-dev
```

### Stable Release Flow

```bash
# 1. Release Team validates prerelease
# Test v2.15-20260716T1430Z-dev in staging

# 2. Release Team triggers manual workflow
# Via GitHub UI:
#   rancher_minor: 2.15
#   release_type: stable

# 3. Workflow creates stable tag at 16:00 UTC
# v2.15-20260805T1600Z created on tested commit

# 4. Build workflow runs
# - Builds with prod refs
# - Pushes images
# - Creates PR to rancher/rancher

# 5. Rancher PR merged
# rancher/rancher picks up v2.15-20260805T1600Z
```

### Querying Release History

```bash
# All stable releases for Rancher 2.15
git tag -l "v2.15-*" --sort=-version:refname | grep -v -- '-dev$'

# Latest stable for Rancher 2.15
git tag -l "v2.15-*" --sort=-version:refname | grep -v -- '-dev$' | head -1

# All prereleases for Rancher 2.14
git tag -l "v2.14-*" --sort=-version:refname | grep -- '-dev$'

# What commit was v2.15-20260805T1600Z built from?
git rev-parse v2.15-20260805T1600Z

# View lock state at v2.15-20260805T1600Z
git show v2.15-20260805T1600Z:lock.yaml
```

## Repository Structure

Each Rancher minor has:
- **Dockerfile** - `dockerfiles/Dockerfile.v2.14`, `dockerfiles/Dockerfile.v2.15`, etc.
- **Config** - Entry in `config.yaml` defining branches
- **Lock state** - Entry in `lock.yaml` tracking commits

## Examples

### Building a Specific Version

```bash
# Prerelease build (uses dev refs from lock.yaml)
make build RANCHER_MINOR=2.15 VERSION=v2.15-20260716T1430Z-dev

# Stable build (uses prod refs from lock.yaml)
make build RANCHER_MINOR=2.15 VERSION=v2.15-20260805T1600Z
```

### Building All Rancher Minors

```bash
# All dev builds with auto-generated CalVer versions
make build-all
# Produces: v2.14-20260716T1430Z-dev, v2.15-20260716T1430Z-dev, ...

# Push all dev builds to registry
make push-all
# Same as build-all but pushes to registry
```

### Fork Versioning

Forks can use the same CalVer scheme with their own registry:

```bash
make push-all \
  REGISTRY=ghcr.io \
  ORG=myorg \
  REPO=my-charts \
  SOURCE_REPO=myorg/rancher-assets
# Produces: ghcr.io/myorg/my-charts:v2.14-20260716T1430Z-dev
```

## Image Labels

All images include OCI labels for traceability:

```dockerfile
org.opencontainers.image.version=${VERSION}
io.rancher.build-type=${BUILD_TYPE}           # "dev" or "prod"
io.rancher.target-branch=${TARGET_BRANCH}     # Rancher branch (release/v2.15)
io.rancher.charts.branch=${CHART_BRANCH}      # Charts branch used
io.rancher.charts.commit=${CHART_COMMIT}      # Charts commit SHA
io.rancher.partner.branch=${PARTNER_BRANCH}   # Partner branch used
io.rancher.partner.commit=${PARTNER_COMMIT}   # Partner commit SHA
io.rancher.rke2.branch=${RKE2_BRANCH}         # RKE2 branch used
io.rancher.rke2.commit=${RKE2_COMMIT}         # RKE2 commit SHA
```

These labels provide full supply chain traceability for every image.

## Summary

**Version tracking:**
- `main` branch: code, config, dockerfiles, lock.yaml (upstream commits only)
- Git tags: CalVer timestamps, point to commits, trigger builds

**Releases:**
- **Auto prereleases**: On merge to main (for changed Rancher minors, with -dev suffix)
- **Manual releases**: Release Team workflow_dispatch (full control over timing)
- **Build on tag**: Automatic build + publish when tag created

**Benefits:**
- ✅ Clean code history (no version bump commits)
- ✅ Rancher version explicit (v2.14-, v2.15- prefix)
- ✅ OOB releases trivial (just new timestamp)
- ✅ No version arithmetic (timestamps self-generate)
- ✅ Flexible control (auto + manual workflows)
- ✅ No workflow loops (tags don't trigger rebuilds)
