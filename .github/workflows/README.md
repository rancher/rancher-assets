# GitHub Workflows Architecture

This repository uses a **two-tier workflow architecture** for clean separation of concerns between tag creation and image building.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ TIER 1: Tag Creation (lock.yaml → CalVer tags)                │
│ Lightweight, local-friendly, just Git operations               │
└─────────────────────────────────────────────────────────────────┘
                              ↓
                        Git tag push
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ TIER 2: Image Building (CalVer tags → Docker images)          │
│ Heavy lifting, multi-arch builds, registry push                │
└─────────────────────────────────────────────────────────────────┘
```

---

## Tier 1: Tag Creation Workflows

### Purpose
Detect changes and create CalVer tags. These workflows are **lightweight** and can be run locally or in CI.

### Workflows

#### 1. `auto-prerelease.yml` - Automatic Pre-releases
- **Trigger:** Push to `main` when `lock.yaml` or `dockerfiles/` change
- **Does:**
  1. Detects which Rancher minors changed
  2. Generates CalVer dev tags (e.g., `v2.15-20260716T1430Z-dev`)
  3. Pushes tags to origin
- **Tags created:** With `-dev` suffix for pre-releases
- **Local equivalent:** `./scripts/create-auto-prerelease.sh`

#### 2. `manual-release.yml` - Manual Releases
- **Trigger:** Manual via GitHub UI (`workflow_dispatch`)
- **Does:**
  1. Accepts Rancher minor and release type
  2. Generates CalVer tags (with or without `-dev` suffix)
  3. Pushes tags to origin
- **Tags created:** With or without `-dev` suffix based on release type
- **Local equivalent:** `make release-manual RELEASE=stable MINOR=2.15`

### Local Execution (Tier 1)

For local tag creation, use the scripts directly:

```bash
# Auto pre-release (detect changes and create dev tags)
./scripts/create-auto-prerelease.sh

# Manual release (stable tags)
make release-manual RELEASE=stable MINOR=2.15

# Manual release (prerelease/dev tags)
make release-manual RELEASE=prerelease MINOR=2.15

# Or use the script directly
./scripts/create-manual-release.sh --release=stable --minor=2.15
```

**Why local?** Tag creation is just Git operations - no Docker builds required. Release Team can create tags locally and push them, triggering CI builds.

---

## Tier 2: Image Building Workflow

### Purpose
Build and publish multi-arch Docker images when tags are created. This is **heavy** and best run in CI.

### Workflow

#### `release.yml` - Build and Release
- **Trigger:** Tag push matching `v*` (any CalVer tag)
- **Does:**
  1. Parses CalVer tag to extract Rancher minor and build type
  2. Selects correct Dockerfile (prod or dev variant)
  3. Builds multi-arch Docker image (`linux/amd64`, `linux/arm64`)
  4. Pushes image to `ghcr.io`
  5. Generates image lists
  6. Creates GitHub release
  7. (Optional) Creates PR to `rancher/rancher` for stable releases

**Tag format detection:**
- `v2.15-20260716T1430Z-dev` → Dev build using `Dockerfile.2.15-dev`
- `v2.14-20260805T1600Z` → Prod build using `Dockerfile.2.14`

### Local Testing (Tier 2)

While you can test image builds locally, the full workflow is designed for CI:

```bash
# Local build test (single platform)
make build RANCHER_MINOR=2.15 VERSION=v2.15-20260716T1430Z-dev

# Build all minors with dev versions
make build-all
```

**Note:** Multi-arch builds and registry pushes should happen in CI.

---

## CalVer Tag Format

All tags follow **Calendar Versioning** with Rancher-minor prefixes:

**Format:** `v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`

**Examples:**
- `v2.14-20260805T1600Z` - Stable release for Rancher 2.14
- `v2.15-20260716T1430Z-dev` - Pre-release for Rancher 2.15

**Components:**
- `v2.15` - Rancher minor version (explicit correlation)
- `20260716` - Date: July 16, 2026
- `T1430Z` - Time: 14:30 UTC (ISO 8601)
- `-dev` - Optional suffix for pre-releases

---

## Workflow Triggers Summary

| Workflow | Trigger | Creates Tags | Builds Images |
|----------|---------|--------------|---------------|
| `auto-prerelease.yml` | Push to `main` (lock.yaml changes) | ✅ Dev tags | ❌ (triggers Tier 2) |
| `manual-release.yml` | Manual (GitHub UI) | ✅ Stable or Dev | ❌ (triggers Tier 2) |
| `release.yml` | Tag push (`v*`) | ❌ | ✅ Multi-arch images |

**Local scripts:**
- `scripts/create-auto-prerelease.sh` → Auto dev tags
- `scripts/create-manual-release.sh` → Manual stable/dev tags

---

## Complete Release Flow

### Automatic Pre-release (Chart Team)

```
1. Chart Team updates lock.yaml (via make generate)
   ↓
2. PR merged to main
   ↓
3. auto-prerelease.yml detects changes
   ↓
4. CalVer dev tag created: v2.15-20260716T1430Z-dev
   ↓
5. release.yml triggered by tag
   ↓
6. Docker image built and pushed
   ↓
7. GitHub release created
```

### Manual Stable Release (Release Team)

```
1. Release Team verifies lock.yaml state
   ↓
2. Runs locally: make release-manual RELEASE=stable MINOR=2.15
   OR runs via GitHub UI: manual-release.yml
   ↓
3. CalVer stable tag created: v2.15-20260805T1600Z
   ↓
4. release.yml triggered by tag
   ↓
5. Docker image built and pushed
   ↓
6. GitHub release created
   ↓
7. PR created to rancher/rancher (stable only)
```

---

## Why Two Tiers?

**Separation of Concerns:**
- **Tier 1:** Lightweight decision-making (which Rancher minors changed?)
- **Tier 2:** Heavy execution (multi-arch Docker builds)

**Local-Friendly:**
- **Tier 1:** Can run anywhere (just Git + Go)
- **Tier 2:** Requires Docker buildx, multi-arch support, registry access

**Flexibility:**
- Tags can be created locally and pushed manually
- CI automatically handles the heavy Docker builds
- Release Team controls when stable tags are created

**Testability:**
- Tier 1 scripts can be tested without Docker
- Tier 2 workflow only runs when tags exist

---

## Migration from SemVer

**Old system:**
- Chart majors (v0, v1) → Rancher minors (2.14, 2.15)
- SemVer tags (v1.0.0, v1.0.0-rc.1) → CalVer (v2.15-20260716T1430Z-dev)
- Versions branch → Git tags as source of truth
- RC number bumping → Timestamp generation

**New system:**
- Explicit Rancher minor in tag
- No version arithmetic (timestamps self-generate)
- No orphan branch maintenance
- OOB releases trivial (just new timestamp)

---

## FAQ

### Q: When should I use `auto-prerelease.yml` vs `manual-release.yml`?

- **Auto:** For Chart Team updates when lock.yaml changes (dev tags)
- **Manual:** For Release Team promotions to stable (stable tags)

### Q: Can I create tags locally instead of using workflows?

**Yes!** Use the scripts:
```bash
./scripts/create-manual-release.sh --release=stable --minor=2.15
```

This is the **recommended** approach for Release Team members.

### Q: What happens if I push a tag manually?

`release.yml` (Tier 2) will automatically trigger and build the image. Just make sure your tag follows the CalVer format.

### Q: How do I test a workflow change?

- **Tier 1:** Run the script locally (`./scripts/create-auto-prerelease.sh --from=HEAD~1 --to=HEAD`)
- **Tier 2:** Create a test tag locally and push to a fork

### Q: What if I need to rebuild an existing tag?

Delete the tag locally and remotely, then recreate it:
```bash
git tag -d v2.15-20260716T1430Z-dev
git push origin :refs/tags/v2.15-20260716T1430Z-dev
# Recreate with same name (not recommended - use new timestamp instead)
```

**Better approach:** Create a new tag with a new timestamp (CalVer makes this trivial).

---

## Troubleshooting

### Workflow didn't trigger

- **Check:** Does your tag match `v*` pattern?
- **Check:** Did the tag actually push to origin? (`git ls-remote --tags origin`)

### Build failed with "Invalid CalVer tag format"

- **Check:** Tag format: `v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`
- **Check:** Rancher minor exists in `config.yaml`

### Auto-prerelease didn't create tags

- **Check:** Did `lock.yaml` or `dockerfiles/` actually change?
- **Check:** Workflow logs in GitHub Actions

### Image has wrong commits

- **Check:** Was `make generate` run before creating the tag?
- **Check:** Inspect lock.yaml at the tag: `git show TAG:lock.yaml`
