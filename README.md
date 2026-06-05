# rancher-assets

This repository builds and publishes the `rancher/rancher-charts` container image, which bundles Helm charts from three upstream repositories for use in air-gapped Rancher deployments.

## Overview

The `rancher-charts` image packages charts from:
- **rancher/charts** - Core Rancher charts
- **rancher/partner-charts** - Partner and ecosystem charts
- **rancher/rke2-charts** - RKE2 system charts

### Why a Separate Repository?

Previously built in `rancher/rancher`, this caused a chicken-and-egg problem where the repository built an asset it also consumed. This new repository:
- Treats all 3 chart repos as equal upstreams
- Enables independent chart releases decoupled from Rancher development
- Simplifies head builds (automatic rebuilds when upstream charts change)
- Provides clear ownership of the charts image lifecycle

## Architecture

**Generator-based approach** (inspired by `rancher/ci-image`):
- Static configuration in `config.yaml` defines chart versions
- Go generator creates Dockerfiles from templates
- Build type detection from tag format (semver vs prerelease)
- Dynamic state tracked in `lock.yaml`
- **Commit-based reproducibility**: Dockerfiles pin specific upstream commits

**Mono-branch strategy**:
- Single `main` branch (no release branches)
- Tag format determines production vs development builds
- Minimal backport overhead

**Reproducible Builds**:
- Generator queries upstream repos for latest commit SHAs
- Commit SHAs are baked into Dockerfiles as ARG defaults
- Builds use specific commits, not moving branch heads
- `git checkout <tag>` → `make build` always produces identical images
- Branch names stored as ENV vars for container metadata

## Usage in Init Containers

The image includes a built-in script at `/usr/local/bin/copy-charts` that copies charts to a shared volume. No command needed:

```yaml
initContainers:
  - name: charts-copy
    image: rancher/rancher-charts:v1.0.0
    volumeMounts:
      - name: charts
        mountPath: /charts
```

**Environment variables** (optional):
- `CHARTS_SOURCE_DIR` - Source directory (default: `/var/lib/rancher-data/local-catalogs/v2`)
- `CHARTS_DEST_DIR` - Destination directory (default: `/charts`)

The script automatically displays metadata (branch/commit) from ENV vars and validates the copy operation.

## Quick Start

### Prerequisites

- Go 1.21+
- Docker with buildx
- yq (for Makefile)
- git

### Generate Dockerfiles

```bash
make generate
```

This will:
1. Load `config.yaml`
2. Query upstream chart repos for latest commits
3. Generate `dockerfiles/Dockerfile.v0` and `dockerfiles/Dockerfile.v1`
4. Update `lock.yaml` with upstream commit references

### Build Chart Images

**Development build (uses dev branches):**
```bash
make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1
```

**Production build (uses prod branches):**
```bash
make build CHART_MAJOR=v1 VERSION=v1.0.0
```

**Build for Rancher 2.14:**
```bash
make build CHART_MAJOR=v0 VERSION=v0.1.0-rc.1
```

### Verify Generated Files

```bash
make verify
```

Ensures all generated files (`dockerfiles/`, `lock.yaml`) are committed.

## Configuration

### config.yaml (Static)

Human-edited configuration defining chart versions and upstream branches.

**When to modify:**
- Adding new Rancher minor version (e.g., v2 for Rancher 2.16)
- Updating base image versions
- Changing upstream repository URLs

**Do NOT modify for:**
- Upstream branch changes (these are managed via CI)
- Build configuration (handled by generator)

### lock.yaml (Dynamic)

Auto-updated by generator and CI. Tracks:
- Latest stable/prerelease versions
- Upstream commit references (used for reproducible builds)
- Copy script hash (SHA256 of `package/copy-charts.sh`)
- Generation timestamps

**When updated:**
- `make generate` - Queries upstreams, updates commit SHAs, computes script hash
- CI after release - Updates latest-stable or latest-prerelease
- Head builds - Refreshes upstream commit references

**Critical for reproducibility:**
The commit SHAs and script hash in lock.yaml are baked into Dockerfiles during `make generate`. This ensures that building from the same repository commit always produces identical chart images, regardless of when the build occurs.

## Build Type Detection

Tag format determines which upstream branches are used:

| Tag Format | Build Type | Charts Branch | Partner/RKE2 Branch |
|-----------|-----------|--------------|-------------------|
| `v1.0.0` | prod | `release-v2.15` | `main` |
| `v1.0.0-rc.1` | dev | `dev-v2.15` | `main` |
| `v1.0.0-dev+abc` | dev | `dev-v2.15` | `main` |

**Clean semver** (e.g., `v1.0.0`) = production build  
**Any prerelease** (e.g., `v1.0.0-rc.1`, `v1.0.0-alpha.1`) = development build

### How Reproducibility Works

**During generation** (`make generate`):
1. Generator queries upstream repos for branch heads
2. Stores commit SHAs in lock.yaml
3. Bakes those commit SHAs into Dockerfiles as ARG defaults

**During build** (`make build`):
1. Makefile reads commit SHAs from lock.yaml
2. Passes them as build args (or uses Dockerfile defaults)
3. Dockerfile clones specific commits, not branch heads
4. Branch names stored as ENV vars for metadata/debugging

**Result**: Building from the same rancher-assets commit always uses the same upstream chart commits, ensuring reproducible images.

## Versioning Strategy

Chart major versions align with Rancher minor releases:

- **v0.x** → Rancher 2.14.x
- **v1.x** → Rancher 2.15.x
- **v2.x** → Rancher 2.16.x (future)

Each chart major version independently tracks its own semver lifecycle.

## Repository Structure

```
rancher-assets/
├── config.yaml              # Static configuration
├── lock.yaml                # Dynamic state tracking
├── Makefile                 # Build automation
├── main.go                  # Generator CLI
├── package/
│   └── copy-charts.sh      # Init container script (copied into images)
├── internal/
│   ├── config/             # Config loading & validation
│   ├── lockfile/           # Lock file management
│   └── generator/          # Dockerfile template rendering
└── dockerfiles/            # Generated Dockerfiles (committed)
    ├── Dockerfile.v0
    └── Dockerfile.v1
```

## Development Workflow

1. **Make changes to config.yaml** (if needed)
2. **Update package/copy-charts.sh** (if modifying init container behavior)
3. **Regenerate Dockerfiles:**
   ```bash
   make generate
   ```
   This will:
   - Query upstream repos for latest commits
   - Compute copy-charts.sh hash
   - Generate Dockerfiles with pinned commits
   - Update lock.yaml
4. **Review changes:**
   ```bash
   git diff dockerfiles/ lock.yaml package/
   ```
5. **Test build locally:**
   ```bash
   make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1
   ```
6. **Test init container script:**
   ```bash
   docker run --rm -v /tmp/charts-test:/charts \
     rancher/rancher-charts:v1.0.0-rc.1
   # Verify charts copied to /tmp/charts-test
   ```
7. **Commit generated files:**
   ```bash
   git add dockerfiles/ lock.yaml config.yaml package/
   git commit -m "chore: regenerate Dockerfiles for v1"
   ```

## Future Work

This initial implementation provides the core generator and local build capability. Future enhancements:

- **GitHub Actions workflows** - Automated release builds, head builds, auto-PR creation
- **Integration with rancher/rancher** - Auto-PR to update `build.yaml` on release
- **Upstream webhooks** - Automatic rebuilds when chart repos change
- **Image list generation** - Extract chart image references for air-gap support
- **Multi-registry publishing** - Docker Hub, Prime staging/production

## Support

For issues or questions, open an issue in this repository.

## License

Copyright (c) 2026 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
