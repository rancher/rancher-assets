# GitHub Actions Workflow Scripts

**Primary location for all CI/CD automation scripts.**

Scripts in this directory are the canonical implementations used by GitHub Actions workflows. If local development or Makefile needs the same functionality, they should call these scripts directly rather than duplicating logic.

## Core Workflow Scripts

### Change Detection

**`detect-changed-dockerfiles.sh`** - Detects which Dockerfiles changed between git refs
- Used by: `pr-smoke-test.yml`, `auto-release.yml`

**`parse-dockerfile-name.sh`** - Extracts Rancher minor and build type from Dockerfile paths
- Used by: `pr-smoke-test.yml`, `auto-release.yml`

### Versioning

**`generate-calver-tag.sh`** - Generates CalVer tags with format `v{MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`
- Used by: `pr-smoke-test.yml`, `auto-release.yml`

**`create-tag.sh`** - Creates annotated git tags with metadata
- Used by: `auto-release.yml`

### Build & Release

**`build-image.sh`** - Builds Docker images (with or without push)
- Used by: `pr-smoke-test.yml`, `release.yml`
- Can be called by Makefile if needed

**`generate-release-notes.sh`** - Generates release notes from metadata
- Used by: `release.yml`

**`generate-rancher-pr-body.sh`** - Generates PR body for rancher/rancher updates
- Used by: `release.yml`

## Usage from Makefile

If Makefile needs workflow functionality, call these scripts directly:

```makefile
build:
	@./.github/scripts/build-image.sh $(DOCKERFILE) $(VERSION)
```

## Usage from Local Development

Developers can call these scripts directly for testing:

```bash
# Detect changes
./.github/scripts/detect-changed-dockerfiles.sh origin/main HEAD

# Build an image
./.github/scripts/build-image.sh dockerfiles/Dockerfile.2.14 my-test-tag

# Generate a tag
./.github/scripts/generate-calver-tag.sh 2.14 prod $(git rev-parse HEAD)
```

## Design Principle

**`.github/scripts/` = Source of Truth**

- Primary implementations live here
- Workflows call these directly
- Makefile can call these directly
- `scripts/` directory is for local dev one-offs only
- No duplication between directories
