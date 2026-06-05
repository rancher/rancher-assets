# Background and Design

This document explains the architecture and design decisions for the rancher-assets build system.

## Problem Statement

The `rancher-assets` image needs to bundle Helm charts from multiple upstream repositories for air-gapped Rancher deployments. Building this inside `rancher/rancher` would create a circular dependency problem:
- Rancher would consume the charts image as a dependency
- Rancher would also build the charts image
- This chicken-and-egg problem would complicate versioning and releases

## Solution

This repository is designed as a standalone build system that:
- Treats all 3 chart repos (`charts`, `partner-charts`, `rke2-charts`) as external upstreams
- Enables independent chart releases decoupled from Rancher development cycles
- Supports automatic rebuilds when upstream charts change
- Provides clear ownership of the charts image lifecycle

## Architecture Decisions

### Generator-Based Approach

Inspired by `rancher/ci-image`, the system uses code generation instead of hand-written Dockerfiles:

**Why:**
- Consistent structure across all chart majors
- Easy to add new chart majors (just update config.yaml)
- Upstream branches and commits baked into generated files
- Single source of truth (config.yaml) for all versions

**How it works:**
1. Static configuration in `config.yaml` defines chart versions and upstream branches
2. Go generator queries upstream repos for latest commits
3. Generator renders Dockerfile templates with pinned commits
4. Generated Dockerfiles committed to git for transparency
5. `lock.yaml` tracks dynamic state (commits, timestamps, script hash)

### Mono-Branch Strategy

The system uses a single `main` branch instead of release branches:

**Why:**
- Minimal backport overhead (no juggling multiple branches)
- Tag format determines build type (prod vs dev)
- All chart majors evolve together on main
- Simpler CI/CD (one branch to watch)

**Trade-offs:**
- Can't have divergent code per Rancher version
- Must maintain backwards compatibility
- Benefits outweigh costs for this use case (simple Docker image packaging)

### Orphan Branch for Version Tracking

Versions are tracked on an orphan `versions` branch instead of in code history:

**Why:**
- Version bumps don't clutter code history on main
- Batch releases possible (multiple chart majors, one commit)
- Clean audit trail (version history separate from code changes)
- No workflow loops (version updates don't trigger rebuilds)

**How it works:**
- `versions` branch contains only `versions.yaml`
- Workflows read current versions, bump them, create tags
- Workflows update `versions` branch after tagging
- `versions` branch never merges to main (orphan)

See [VERSION.md](VERSION.md) for complete versioning strategy.

### Reproducible Builds

The design ensures every build from the same git tag produces identical images, regardless of when built:

**How we achieve this:**

1. **Commit pinning:**
   - `make generate` queries upstream repos for branch head commits
   - Commit SHAs stored in `lock.yaml`
   - Commit SHAs baked into Dockerfiles as ARG defaults
   - Dockerfiles clone specific commits, not moving branch heads

2. **Script hashing:**
   - `package/copy-charts.sh` hash computed during generation
   - Hash stored in `lock.yaml`
   - Ensures script changes trigger new builds

3. **Immutable lock file:**
   - `lock.yaml` committed to git
   - Checking out a tag gives you the exact lock state from that release

**Example scenario:**
```bash
# Build v1.0.0 today
git checkout v1.0.0
make build CHART_MAJOR=v1 VERSION=v1.0.0
# Image digest: sha256:abc123...

# Rebuild v1.0.0 6 months later
# (upstream branches have moved, but commits are pinned in lock.yaml)
git checkout v1.0.0
make build CHART_MAJOR=v1 VERSION=v1.0.0
# Image digest: sha256:abc123... (identical!)
```

### Build Type Detection

The tag format determines which upstream refs are used:

| Tag Format | Build Type | Upstream Refs Used | Use Case |
|-----------|-----------|-------------------|----------|
| `v1.0.0` | prod | `lock.yaml` → `upstream-refs.prod` | Stable releases |
| `v1.0.0-rc.1` | dev | `lock.yaml` → `upstream-refs.dev` | Pre-releases, testing |

**Why this matters:**
- Production builds use stable upstream branches (`release-v2.15`)
- Development builds use dev upstream branches (`dev-v2.15`)
- Same codebase, different upstream refs
- Build args override Dockerfile defaults for prod builds

**Implementation:**
```bash
# Makefile detects build type from VERSION tag
if [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  BUILD_TYPE=prod
  # Pass prod commits as build args
else
  BUILD_TYPE=dev
  # Use Dockerfile defaults (dev commits)
fi
```

### Chart Major Versioning

Chart major versions align with Rancher minor versions:

| Chart Major | Rancher Version | Rationale |
|-------------|-----------------|-----------|
| v0.x | 2.14.x | Charts for Rancher 2.14 |
| v1.x | 2.15.x | Charts for Rancher 2.15 |
| v2.x | 2.16.x | Charts for Rancher 2.16 (future) |

**Why chart majors:**
- Each Rancher minor has different chart requirements
- Chart versions can evolve independently per Rancher version
- Breaking changes scoped to chart major (Rancher minor)
- Clear compatibility story (use v1.x charts with Rancher 2.15)

**Why not Rancher version as image tag:**
- Rancher 2.15 might have many chart releases (2.15.0, 2.15.1, etc.)
- Chart releases decoupled from Rancher releases
- Chart versioning independent of Rancher versioning

## Supply Chain Traceability

The design includes comprehensive OCI labels for full traceability:

```dockerfile
# Standard OCI labels
org.opencontainers.image.version=${VERSION}
org.opencontainers.image.created=${BUILD_DATE}
org.opencontainers.image.source=https://github.com/rancher/rancher-assets

# Rancher-specific attestation
io.rancher.build-type=${BUILD_TYPE}           # "dev" or "prod"
io.rancher.target-branch=${TARGET_BRANCH}     # Rancher branch
io.rancher.charts.branch=${CHART_BRANCH}      # Charts branch used
io.rancher.charts.commit=${CHART_COMMIT}      # Charts commit SHA
io.rancher.partner.branch=${PARTNER_BRANCH}   # Partner branch used
io.rancher.partner.commit=${PARTNER_COMMIT}   # Partner commit SHA
io.rancher.rke2.branch=${RKE2_BRANCH}         # RKE2 branch used
io.rancher.rke2.commit=${RKE2_COMMIT}         # RKE2 commit SHA
```

**Why:**
- Provides full audit trail for compliance
- Enables debugging by inspecting image metadata
- Allows tracing back to exact upstream commits used
- Verifies build inputs without rebuilding

**Inspecting labels:**
```bash
docker inspect ghcr.io/rancher/rancher-assets:v1.0.0 | jq '.[0].Config.Labels'
```

## Configuration Files

### config.yaml (Static)

Human-edited, changes rarely:

```yaml
base-image:
  bci-version: "16.0"

chart-versions:
  v1:
    rancher-branch: release/v2.15
    prod:
      charts-branch: release-v2.15
      partner-branch: main
      rke2-branch: main
    dev:
      charts-branch: dev-v2.15
      partner-branch: main
      rke2-branch: main
```

**When it changes:**
- Adding new Rancher minor (new chart major)
- Updating base image version
- Changing upstream branch names

### lock.yaml (Dynamic)

Auto-generated, changes frequently:

```yaml
copy-script-hash: sha256:abc123...
chart-versions:
  v1:
    upstream-refs:
      prod:
        charts: {branch: release-v2.15, commit: f408a794..., fetched-at: "2026-06-05T10:00:00Z"}
        partner: {branch: main, commit: 13b90384..., fetched-at: "2026-06-05T10:00:00Z"}
        rke2: {branch: main, commit: d0865878..., fetched-at: "2026-06-05T10:00:00Z"}
      dev:
        charts: {branch: dev-v2.15, commit: a7b9d818..., fetched-at: "2026-06-05T10:00:00Z"}
        partner: {branch: main, commit: 13b90384..., fetched-at: "2026-06-05T10:00:00Z"}
        rke2: {branch: main, commit: d0865878..., fetched-at: "2026-06-05T10:00:00Z"}
```

**When it changes:**
- `make generate` queries upstreams and updates commits
- Never edited manually

**Why both files:**
- `config.yaml` = intent (what branches to track)
- `lock.yaml` = state (what commits are current)
- Separation of concerns

## Init Container Pattern

The images are designed to run as init containers in Rancher:

```yaml
initContainers:
  - name: charts-copy
    image: ghcr.io/rancher/rancher-assets:v1.0.0
    volumeMounts:
      - name: charts
        mountPath: /charts
```

**How it works:**
1. Image contains `/usr/local/bin/copy-charts` script
2. Dockerfile sets `CMD ["/usr/local/bin/copy-charts"]`
3. Script copies from `/var/lib/rancher-data/local-catalogs/v2` to `/charts`
4. Script displays metadata (branches/commits) from ENV vars
5. Script validates copy operation
6. Init container exits, main container starts

**Why this pattern:**
- Immutable chart bundles (container image)
- Declarative consumption (Kubernetes YAML)
- Version pinning (image tag)
- No external network access needed (air-gap friendly)

## Workflow Automation

The system includes three GitHub Actions workflows to handle releases:

### 1. auto-prerelease.yml

**Trigger:** Push to main when `lock.yaml` changes

**Why lock.yaml:**
- Source of truth for all build inputs
- Upstream commit refs (what charts are bundled)
- Package script hash (how charts are copied)
- Generated by `make generate` which runs on config/package/upstream changes

**What it does:**
1. Detects which chart majors have upstream ref changes (ignores timestamps)
2. Reads current versions from `versions` branch
3. Auto-bumps prerelease versions (v1.0.0-rc.1 → v1.0.0-rc.2)
4. Creates git tags on merge commit
5. Updates `versions` branch

**Change detection:**
Uses Go tool (`go run main.go changed-majors --from=HEAD^ --to=HEAD`) to diff lock.yaml and output JSON array of changed majors.

### 2. manual-release.yml

**Trigger:** Manual workflow_dispatch

**Inputs:**
- `commit_sha` - Commit to release (default: HEAD)
- `chart_major` - Which chart major (empty = ALL)
- `bump_type` - minor or patch
- `release_type` - prerelease (default) or stable

**Use cases:**
- Stable releases (Release Team promotion)
- Batch releases (all chart majors from one commit)
- Selective releases (only v1, only v0, etc.)

### 3. build-release.yml

**Trigger:** Tag matching `v*` pattern

**What it does:**
1. Parses tag to determine version and build type
2. Builds multi-arch images (linux/amd64, linux/arm64)
3. Pushes to Docker Hub
4. Creates GitHub Release with metadata
5. Creates PR to `rancher/rancher` (stable releases only)

See [VERSION.md](VERSION.md) for workflow details and [RELEASE-TEAM-GUIDE.md](RELEASE-TEAM-GUIDE.md) for operations.

## Fork-Friendly Design

The system supports customization via Makefile variables:

```bash
make push-all \
  REGISTRY=ghcr.io \
  ORG=myorg \
  REPO=my-charts \
  SOURCE_REPO=myorg/rancher-assets
```

**Defaults:**
- `REGISTRY=ghcr.io`
- `ORG=rancher`
- `REPO=rancher-assets`
- `SOURCE_REPO=rancher/rancher-assets`

**Why:**
- Community forks can publish to their own registries
- Internal deployments can use private registries
- No code changes needed, just override variables

## Multi-Arch Support

The build system produces multi-arch manifests (single tag, multiple platforms):

```bash
docker buildx build --platform linux/amd64,linux/arm64 ...
```

**Why:**
- ARM support for edge deployments
- Single image tag works on both architectures
- Docker automatically pulls correct platform

**Trade-offs:**
- Requires buildx (more complex than simple builds)
- Longer build times (building for 2 platforms)
- Worth it for Rancher's multi-arch requirements

## Design Trade-offs

### Committed Dockerfiles

**Decision:** Commit generated Dockerfiles to git

**Why:**
- Transparency (see exactly what will build)
- CI validation (`make verify` fails if not generated)
- Git history shows what changed over time
- No generation step needed for read-only operations

**Trade-off:**
- Generated files in git (code review noise)
- Must remember to run `make generate`
- Worth it for transparency and auditability

### Lock File Structure

**Decision:** Track both prod and dev refs in lock.yaml

**Why:**
- Single source of truth for all build inputs
- Generator queries both sets of branches once
- Dockerfiles default to dev refs
- Build args override for prod builds

**Trade-off:**
- Larger lock file
- More complex structure
- Clearer than separate files

### Orphan Branch

**Decision:** Use orphan branch instead of tags-only

**Why:**
- Version bumps don't clutter main history
- Can batch multiple chart majors in one version update
- Clean audit trail for versions
- No workflow loops (version updates don't retrigger builds)

**Trade-off:**
- Two branches to manage (main + versions)
- More complex than single-branch
- Benefits outweigh complexity

## Design Priorities

The system is designed to prioritize:
- **Reproducibility** - Same tag always builds same image
- **Traceability** - Full supply chain attestation via labels
- **Automation** - Minimal manual steps, workflows handle releases
- **Simplicity** - Mono-branch, generator pattern, clear separation of concerns
- **Fork-friendly** - Easy to customize for different registries/orgs
