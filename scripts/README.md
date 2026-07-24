# Local Development Scripts

Scripts for local development, one-off engineering tasks, and Makefile-specific helpers that aren't appropriate for GitHub Actions.

## When to Use This Directory

Add scripts here when they are:
- Local development utilities
- One-off engineering tasks
- Makefile-specific helpers that don't belong in workflows
- NOT used by GitHub Actions

## When NOT to Use This Directory

If a script is used by GitHub Actions workflows, it belongs in `.github/scripts/` instead. Makefile can call `.github/scripts/` directly if needed.

## Scripts in This Directory

### Build Utilities

**`export-image-lists.sh`** - Extracts image lists from Docker images
- Used by: `make export-images`, workflows (via direct call)
- Generates Linux and Windows image lists for air-gapped deployments

**`get-build-vars.sh`** - Extracts build variables for a version
- Used by: Makefile (legacy, may be deprecated)

**`build-releases.sh`** - Batch builds multiple releases
- Used by: Local development

**`version`** - Version calculation and git utilities
- Used by: Makefile for computing GIT_COMMIT and TARGET_BRANCH

### Development Utilities

**`vendor-update.sh`** - Updates Go vendor dependencies
- Used by: `make vendor-update`

**`create-manual-release.sh`** - Creates manual release tags
- Used by: `.github/workflows/manual-release.yml`

## Usage

```bash
# Export image lists after building
./scripts/export-image-lists.sh \
  --image ghcr.io/myuser/rancher-assets:v2.14-test \
  --version v2.14-test \
  --output-dir dist/v2.14-test

# Update vendor dependencies
./scripts/vendor-update.sh

# Create a manual release
./scripts/create-manual-release.sh --help
```

## Relationship with `.github/scripts/`

**`.github/scripts/`** = Primary implementations for CI/CD (source of truth)
**`scripts/`** = Local dev helpers and Makefile-specific tools

If you need CI/CD functionality locally, call `.github/scripts/` directly rather than duplicating code here.
