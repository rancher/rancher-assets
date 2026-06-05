# rancher-assets

Builds the `rancher/rancher-assets` container image, which bundles Helm charts repo's used by `ClusterRepos` for air-gapped Rancher deployments.

## What It Does

This repository packages charts from three upstream sources into versioned container images:
- **rancher/charts** - Core Rancher charts
- **rancher/partner-charts** - Partner and ecosystem charts  
- **rancher/rke2-charts** - RKE2 system charts

Each image is a complete, immutable snapshot of chart repositories at specific upstream commits.

## Why

Previously built inside `rancher/rancher`, which created a circular dependency. This standalone repository:
- Treats all chart repos as external dependencies
- Enables independent chart releases
- Provides clear ownership of the charts image lifecycle

## Usage

Use as an init container to copy charts to a shared volume:

```yaml
initContainers:
  - name: charts-copy
    image: ghcr.io/rancher/rancher-assets:v1.0.0
    volumeMounts:
      - name: charts
        mountPath: /charts
```

The image automatically runs `/usr/local/bin/copy-charts` which copies bundled charts to the mounted volume.

## Versioning

Chart major versions align with Rancher minor releases:

| Chart Major | Rancher Version | Status |
|-------------|-----------------|--------|
| v0.x        | 2.14.x          | Active |
| v1.x        | 2.15.x          | Active |

See [VERSION.md](VERSION.md) for complete versioning strategy and release workflows.

## Quick Start

**Generate Dockerfiles from upstream:**
```bash
make generate
```

**Build a specific chart major:**
```bash
make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1
```

**Build all chart majors (development versions):**
```bash
make build-all
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow and contribution guidelines.

## Documentation

- [BACKGROUND.md](BACKGROUND.md) - Architecture, design decisions, technical context
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development workflow, build system, testing
- [VERSION.md](VERSION.md) - Versioning strategy, release workflows, version tracking

## License

Copyright (c) 2026 SUSE LLC

Licensed under the Apache License, Version 2.0. See LICENSE for full text.
