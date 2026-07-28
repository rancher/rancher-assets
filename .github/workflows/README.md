# GitHub Workflows Architecture

This repository uses a **two-tier workflow architecture** for clean separation of concerns between tag creation and image building.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ TIER 1: Tag Creation (Dockerfiles → CalVer tags)              │
│ Automatic tag creation when Dockerfiles change on main         │
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

## Tier 1: Tag Creation Workflow

### Purpose
Automatically detect Dockerfile changes and create CalVer tags.

### Workflow

#### `auto-release.yml` - Automatic Releases
- **Trigger:** Push to `main` when `dockerfiles/**` change
- **Does:**
  1. Detects which specific Dockerfiles changed
  2. Generates CalVer tags (with `-dev` suffix for dev builds)
  3. Pushes tags to origin
- **Tags created:**
  - `v{MINOR}-{TIMESTAMP}` for prod Dockerfiles
  - `v{MINOR}-{TIMESTAMP}-dev` for dev Dockerfiles

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
make build-image TAG=v2.15-20260716T1430Z-dev RANCHER_MINOR=2.15 DEV=true

# Or for a prod build
make build-image TAG=v2.15-20260716T1430Z RANCHER_MINOR=2.15 DEV=false
```

**Note:** Multi-arch builds and registry pushes should happen in CI via `make push-image`.

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
| `auto-release.yml` | Push to `main` (dockerfiles/ changes) | Yes (Prod/Dev tags) | No (triggers Tier 2) |
| `release.yml` | Tag push (`v*`) | No | Yes (Multi-arch images) |

---

## Complete Release Flow

### Automatic Release (On Dockerfile Changes)

```
1. Developer updates lock.yaml (via make generate)
   ↓
2. PR merged to main
   ↓
3. auto-release.yml detects Dockerfile changes
   ↓
4. CalVer tags created for each changed Dockerfile:
   - Prod: v2.15-20260716T1430Z
   - Dev: v2.15-20260716T1430Z-dev
   ↓
5. release.yml triggered by tags (parallel builds)
   ↓
6. Docker images built and pushed
   ↓
7. GitHub releases created
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

### Q: How do releases get created?

Releases are created automatically when Dockerfiles change on the `main` branch. The `auto-release.yml` workflow detects changes and creates CalVer tags, which trigger image builds.

### Q: Can I create tags manually?

You can manually create and push git tags following the CalVer format (`v{MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`), and `release.yml` will automatically trigger and build the image.

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
