# rancher/rancher-assets Repository Design

## Executive Summary

This document defines the architecture for a new `rancher/rancher-assets` repository that will build and publish the `rancher/rancher-charts` container image. This image bundles Helm charts from three upstream repositories (rancher/charts, rancher/partner-charts, rancher/rke2-charts) for use in air-gapped Rancher deployments.

**Key Decisions:**
- **Branching Strategy**: Mono-branch (single `main` branch)
- **Architecture Pattern**: Generator-based (inspired by rancher/ci-image)
- **Versioning**: SemVer with chart major versions aligned to Rancher minor releases
- **Build Detection**: Tag format determines production vs development builds

---

## Table of Contents

1. [Background](#background)
2. [Requirements](#requirements)
3. [Architecture Overview](#architecture-overview)
4. [Repository Structure](#repository-structure)
5. [Configuration System](#configuration-system)
6. [Build System](#build-system)
7. [Release Workflows](#release-workflows)
8. [Head Builds](#head-builds)
9. [Integration with Rancher](#integration-with-rancher)
10. [Migration Path](#migration-path)
11. [Appendix: Alternatives Considered](#appendix-alternatives-considered)

---

## Background

### Current State

The `rancher/rancher-charts` image is currently built from the `rancher/rancher` repository:
- Dockerfile: `package/Dockerfile-charts`
- Workflows: `.github/workflows/release-charts-image.yml`, `.github/workflows/charts-update-pr.yml`
- Configuration: `build.yaml` defines chart branches

**Problems with current approach:**
1. **Chicken-and-egg**: Building charts image in same repo where it's consumed
2. **Coupling**: Charts releases tied to Rancher development cycle
3. **Head build complexity**: Hard to trigger builds when any of 3 upstream chart repos change

### Why a Separate Repository?

**Benefits:**
1. **Equal upstream treatment**: All 3 chart repos treated as equal upstreams
2. **Independent releases**: Charts can be released without Rancher code changes
3. **Simpler head builds**: Single repo watches 3 upstreams and rebuilds as needed
4. **Clear ownership**: Repository dedicated to charts image lifecycle

---

## Requirements

### Functional Requirements

1. **Multi-Version Support**: Support multiple Rancher minor versions simultaneously (2.15, 2.16, etc.)
2. **Dev vs Prod Builds**: Different upstream branch selection for RC vs stable releases
3. **Head Builds**: Automatic builds when upstream chart repos change
4. **Auto-PR Creation**: Automatically create PRs to rancher/rancher when new charts versions are released
5. **Air-Gap Support**: Generate image lists for air-gapped deployments
6. **Multi-Arch**: Support linux/amd64 and linux/arm64

### Non-Functional Requirements

1. **Maintainability**: Minimize manual backport effort
2. **Reproducibility**: Builds from same commit produce identical images
3. **Auditability**: Track what upstream chart commits are bundled
4. **Simplicity**: Avoid branching overhead where possible

---

## Architecture Overview

### High-Level Design

```
┌─────────────────────────────────────────────────────────┐
│         rancher/rancher-assets (mono-branch)            │
│                                                          │
│  ┌────────────┐     ┌──────────────┐                   │
│  │ config.yaml│────▶│  generator   │                   │
│  │  (static)  │     │  (Go code)   │                   │
│  └────────────┘     └──────┬───────┘                   │
│                             │                            │
│                             ▼                            │
│              ┌──────────────────────────┐               │
│              │  Generated Artifacts:    │               │
│              │  - Dockerfile.v1         │               │
│              │  - Dockerfile.v2         │               │
│              │  - lock.yaml (updated)   │               │
│              └──────────────────────────┘               │
│                                                          │
│  Tag: v1.0.0 ──────▶ Build v1 (prod branches)          │
│  Tag: v1.0.0-rc.1 ─▶ Build v1 (dev branches)           │
│  Tag: v2.0.0 ──────▶ Build v2 (prod branches)          │
└─────────────────────────────────────────────────────────┘
                      │
                      ▼
        ┌─────────────────────────────┐
        │  Docker Hub / GHCR          │
        │  rancher/rancher-charts:v1.0.0 │
        │  rancher/rancher-charts:v2.0.0 │
        └─────────────────────────────┘
                      │
                      ▼
        ┌─────────────────────────────┐
        │  Auto-PR to rancher/rancher │
        │  Updates: build.yaml        │
        │          chart/values.yaml  │
        └─────────────────────────────┘
```

### Key Architectural Principles

1. **Generator-Based**: Dockerfiles are generated from config, not hand-written
2. **Config as Truth**: `config.yaml` is the single source of truth
3. **Separation of Concerns**: Static config vs dynamic lock file
4. **Convention over Configuration**: Tag format determines build behavior

---

## Repository Structure

```
rancher-assets/
├── README.md
├── Makefile
├── main.go                    # Generator CLI
├── go.mod
├── go.sum
│
├── config.yaml               # Static configuration (human-edited)
├── lock.yaml                 # Dynamic state (generated + CI-updated)
│
├── internal/
│   ├── config/              # Config loading and validation
│   ├── generator/           # Dockerfile template rendering
│   └── lockfile/            # Lock file management
│
├── dockerfiles/             # Generated (committed for auditability)
│   ├── Dockerfile.v1
│   ├── Dockerfile.v2
│   └── ...
│
└── .github/
    └── workflows/
        ├── pull-request.yml      # Validate PRs
        ├── release.yml           # Tag-triggered builds
        ├── head-build.yml        # Upstream change builds
        └── update-lock.yml       # Periodic lock refresh
```

---

## Configuration System

### config.yaml (Static)

**Purpose**: Human-edited source of truth for chart version configurations

**File**: `config.yaml`

```yaml
# Rancher Assets Configuration
# Defines chart major versions and their upstream source configurations
# This file is STATIC - only changes when adding new Rancher minors

chart-versions:
  # Charts v1.x series for Rancher 2.15.x
  v1:
    # Which rancher/rancher branch this chart major targets
    rancher-branch: release/v2.15
    
    # Production builds (clean semver: v1.0.0, v1.1.0)
    prod:
      charts-branch: release-v2.15
      partner-branch: main-source
      rke2-branch: main-source
    
    # Development builds (prereleases: v1.0.0-rc.1, v1.0.0-dev+sha)
    dev:
      charts-branch: dev-v2.15
      partner-branch: main
      rke2-branch: main
  
  # Charts v2.x series for Rancher 2.16.x
  v2:
    rancher-branch: release/v2.16
    
    prod:
      charts-branch: release-v2.16
      partner-branch: main-source
      rke2-branch: main-source
    
    dev:
      charts-branch: dev-v2.16
      partner-branch: main
      rke2-branch: main

# Base image for all chart builds
base-image:
  bci-base: registry.suse.com/bci/bci-base:15.7
  bci-micro: registry.suse.com/bci/bci-micro:16.0

# Upstream chart repository URLs
repos:
  charts: https://git.rancher.io/charts
  partner: https://git.rancher.io/partner-charts
  rke2: https://git.rancher.io/rke2-charts

# Chart destination paths (hash-based for integrity)
paths:
  charts: /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721
  partner: /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974
  rke2: /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9
```

**When to modify**: Only when adding new Rancher minor version or changing base images

---

### lock.yaml (Dynamic)

**Purpose**: Track latest released versions and upstream state

**File**: `lock.yaml`

```yaml
# Rancher Assets Lock File
# Auto-updated by CI after successful releases
# Tracks latest versions and upstream commit references

chart-versions:
  v1:
    latest-stable: v1.0.0
    latest-prerelease: v1.1.0-rc.1
    updated-at: "2026-01-15T10:30:00Z"
    
    # Upstream commit references (for detecting changes)
    upstream-refs:
      charts:
        branch: release-v2.15
        commit: abc123def456...
        fetched-at: "2026-01-15T10:00:00Z"
      partner:
        branch: main-source
        commit: def456ghi789...
        fetched-at: "2026-01-15T10:00:00Z"
      rke2:
        branch: main-source
        commit: ghi789jkl012...
        fetched-at: "2026-01-15T10:00:00Z"
  
  v2:
    latest-stable: v2.0.0
    latest-prerelease: null
    updated-at: "2026-01-10T14:20:00Z"
    upstream-refs:
      charts:
        branch: release-v2.16
        commit: jkl012mno345...
        fetched-at: "2026-01-10T14:00:00Z"
      partner:
        branch: main-source
        commit: mno345pqr678...
        fetched-at: "2026-01-10T14:00:00Z"
      rke2:
        branch: main-source  
        commit: pqr678stu901...
        fetched-at: "2026-01-10T14:00:00Z"

generated-at: "2026-01-15T10:30:00Z"
generator-version: v0.1.0
```

**When updated**:
- **CI (after release)**: Updates `latest-stable` or `latest-prerelease`
- **make generate**: Updates `upstream-refs` when regenerating
- **Head builds**: Refreshes upstream commit references

---

## Build System

### Generator (Go)

**Command**: `make generate`

**What it does**:
1. Loads `config.yaml`
2. Queries upstream chart repos for latest commit SHAs
3. Generates `dockerfiles/Dockerfile.<major>` for each chart version
4. Updates `lock.yaml` with upstream refs

**Example Generated Dockerfile**:

```dockerfile
# dockerfiles/Dockerfile.v1
# Generated by rancher-assets generator
# DO NOT EDIT - changes will be overwritten

ARG BCI_VERSION=15.7
ARG BCI_MICRO_VERSION=16.0

# Build args for chart branches (set by CI workflow)
ARG CHART_BRANCH=dev-v2.15
ARG PARTNER_BRANCH=main
ARG RKE2_BRANCH=main

# Runtime labels (set by CI workflow)
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
ARG SOURCE_BRANCH
ARG BUILD_URL

FROM registry.suse.com/bci/bci-base:${BCI_VERSION} AS builder
RUN zypper refresh && zypper -n install git-core

# Clone rancher-charts repository
FROM builder AS rancher-charts
ARG CHART_BRANCH
RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    git clone --no-checkout -b ${CHART_BRANCH} --depth 1 \
      https://git.rancher.io/charts \
      /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721

# Clone partner-charts repository
FROM builder AS partner-charts
ARG PARTNER_BRANCH
RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    git clone --no-checkout -b ${PARTNER_BRANCH} --depth 1 \
      https://git.rancher.io/partner-charts \
      /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974

# Clone rke2-charts repository
FROM builder AS rke2-charts
ARG RKE2_BRANCH
RUN mkdir -p /var/lib/rancher-data/local-catalogs/v2 && \
    git config --global url."https://github.com/rancher/".insteadOf https://git.rancher.io/ && \
    git clone --no-checkout -b ${RKE2_BRANCH} --depth 1 \
      https://git.rancher.io/rke2-charts \
      /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9

# Final charts image
FROM registry.suse.com/bci/bci-micro:${BCI_MICRO_VERSION} AS charts

COPY --from=rancher-charts \
  /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721 \
  /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721

COPY --from=partner-charts \
  /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 \
  /var/lib/rancher-data/local-catalogs/v2/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974

COPY --from=rke2-charts \
  /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9 \
  /var/lib/rancher-data/local-catalogs/v2/rancher-rke2-charts/675f1b63a0a83905972dcab2794479ed599a6f41b86cd6193d69472d0fa889c9

LABEL org.opencontainers.image.source="https://github.com/rancher/rancher-assets" \
      org.opencontainers.image.title="Rancher Charts v1.x" \
      org.opencontainers.image.description="Rancher Charts for Rancher 2.15.x" \
      org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.revision=${GIT_COMMIT} \
      org.opencontainers.image.created=${BUILD_DATE} \
      io.rancher.target-branch=${SOURCE_BRANCH} \
      io.rancher.build-url=${BUILD_URL}
```

---

### Makefile

```makefile
SHELL := /bin/bash
.DEFAULT_GOAL := help

DOCKERFILES_DIR := dockerfiles

# Chart major versions from lock.yaml
ALL_VERSIONS := $(shell awk '/^chart-versions:/{f=1;next} /^[a-zA-Z]/{f=0} f && /^ *[a-z0-9]/{gsub(/:/, "", $$1); print $$1}' lock.yaml)

VERSION ?=
CHART_MAJOR ?=
BUILD_TYPE ?=
ORG ?= rancher
REPO ?= rancher-charts
IMAGE_REPO ?= docker.io/$(ORG)/$(REPO)
TARGET_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help generate verify build push

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

generate: ## Generate Dockerfiles from config.yaml
	go run main.go generate

verify: ## Verify no uncommitted changes
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: uncommitted changes detected"; \
		git status --porcelain; \
		git diff; \
		exit 1; \
	fi

# Build a specific chart major version
# Usage: make build CHART_MAJOR=v1 VERSION=v1.0.0
build:
	@if [ -z "$(CHART_MAJOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "Error: CHART_MAJOR and VERSION required"; \
		exit 1; \
	fi
	@# Detect build type from version tag
	@if echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		BUILD_TYPE=prod; \
	else \
		BUILD_TYPE=dev; \
	fi; \
	echo "Building $(CHART_MAJOR) version $(VERSION) ($$BUILD_TYPE)"; \
	# Read branches from config based on chart major and build type
	CHART_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".$$BUILD_TYPE.charts-branch" config.yaml); \
	PARTNER_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".$$BUILD_TYPE.partner-branch" config.yaml); \
	RKE2_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".$$BUILD_TYPE.rke2-branch" config.yaml); \
	RANCHER_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".rancher-branch" config.yaml); \
	docker buildx build \
		--file "$(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)" \
		--platform "$(TARGET_PLATFORMS)" \
		--build-arg CHART_BRANCH=$$CHART_BRANCH \
		--build-arg PARTNER_BRANCH=$$PARTNER_BRANCH \
		--build-arg RKE2_BRANCH=$$RKE2_BRANCH \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$$(git rev-parse HEAD) \
		--build-arg BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		--build-arg SOURCE_BRANCH=$$RANCHER_BRANCH \
		--label io.rancher.target-branch=$$RANCHER_BRANCH \
		--label io.rancher.chart-major=$(CHART_MAJOR) \
		--label io.rancher.build-type=$$BUILD_TYPE \
		--tag "$(IMAGE_REPO):$(VERSION)" \
		.

push: build ## Build and push
	docker push "$(IMAGE_REPO):$(VERSION)"

test: ## Run tests
	go test -v ./...
```

---

## Release Workflows

### 1. Tag-Based Release

**Trigger**: Tag matching `v*` pattern (e.g., `v1.0.0`, `v1.0.0-rc.1`, `v2.0.0`)

**Workflow**: `.github/workflows/release.yml`

```yaml
name: Release Charts Image

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write
  packages: write

jobs:
  extract-metadata:
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.parse.outputs.version }}
      major: ${{ steps.parse.outputs.major }}
      build_type: ${{ steps.parse.outputs.build_type }}
      rancher_branch: ${{ steps.config.outputs.rancher_branch }}
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Parse tag
        id: parse
        run: |
          VERSION=${GITHUB_REF#refs/tags/}
          MAJOR=$(echo $VERSION | cut -d. -f1)
          
          # Detect build type from tag format
          if [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            BUILD_TYPE=prod
            echo "Clean semver detected - PROD build"
          else
            BUILD_TYPE=dev
            echo "Prerelease detected - DEV build"
          fi
          
          echo "version=$VERSION" >> $GITHUB_OUTPUT
          echo "major=$MAJOR" >> $GITHUB_OUTPUT
          echo "build_type=$BUILD_TYPE" >> $GITHUB_OUTPUT
      
      - name: Load config
        id: config
        run: |
          MAJOR=${{ steps.parse.outputs.major }}
          RANCHER_BRANCH=$(yq eval ".chart-versions.\"${MAJOR}\".rancher-branch" config.yaml)
          
          if [ "$RANCHER_BRANCH" = "null" ]; then
            echo "ERROR: No config found for chart major $MAJOR"
            exit 1
          fi
          
          echo "rancher_branch=$RANCHER_BRANCH" >> $GITHUB_OUTPUT

  build-charts-image:
    needs: extract-metadata
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          - tag-suffix: ""
            platforms: linux/amd64,linux/arm64
          - tag-suffix: "-amd64"
            platforms: linux/amd64
          - tag-suffix: "-arm64"
            platforms: linux/arm64
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Setup Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}
      
      - name: Build and push
        run: |
          make push \
            CHART_MAJOR=${{ needs.extract-metadata.outputs.major }} \
            VERSION=${{ needs.extract-metadata.outputs.version }}${{ matrix.tag-suffix }} \
            TARGET_PLATFORMS=${{ matrix.platforms }}
  
  update-lock-file:
    needs: [extract-metadata, build-charts-image]
    if: success()
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Update lock.yaml
        run: |
          MAJOR=${{ needs.extract-metadata.outputs.major }}
          VERSION=${{ needs.extract-metadata.outputs.version }}
          BUILD_TYPE=${{ needs.extract-metadata.outputs.build_type }}
          TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
          
          # Update appropriate field based on build type
          if [ "$BUILD_TYPE" = "prod" ]; then
            yq eval ".chart-versions.\"${MAJOR}\".latest-stable = \"${VERSION}\"" -i lock.yaml
          else
            yq eval ".chart-versions.\"${MAJOR}\".latest-prerelease = \"${VERSION}\"" -i lock.yaml
          fi
          
          yq eval ".chart-versions.\"${MAJOR}\".updated-at = \"${TIMESTAMP}\"" -i lock.yaml
      
      - name: Commit and push
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add lock.yaml
          git commit -m "chore: update lock.yaml for ${{ needs.extract-metadata.outputs.version }}"
          git push
  
  create-rancher-pr:
    needs: [extract-metadata, build-charts-image]
    if: success() && needs.extract-metadata.outputs.build_type == 'prod'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Create PR to rancher/rancher
        run: |
          gh pr create --repo rancher/rancher \
            --base ${{ needs.extract-metadata.outputs.rancher_branch }} \
            --title "Update to rancher-charts ${{ needs.extract-metadata.outputs.version }}" \
            --body "Automated PR: Updates rancher-charts image to ${{ needs.extract-metadata.outputs.version }}"
        env:
          GITHUB_TOKEN: ${{ secrets.RANCHER_REPO_TOKEN }}
```

**Build Type Detection**:
- `v1.0.0` (clean semver) → **PROD** build → uses `prod.charts-branch`, `prod.partner-branch`, etc.
- `v1.0.0-rc.1` (prerelease) → **DEV** build → uses `dev.charts-branch`, `dev.partner-branch`, etc.
- `v1.0.0-dev+abc123` (dev tag) → **DEV** build

**Stable Release Auto-PR**:
- Only PROD builds trigger PR creation
- DEV/RC builds skip this step (manual testing)

---

### 2. Head Builds

**Purpose**: Build charts images when upstream chart repos change, rebuilding ONLY affected chart majors

**Key Design Principle**: Smart detection ensures that only chart majors using the changed upstream branch are rebuilt, not all images.

#### Trigger Mechanisms

**1. Automated (Production)**: Repository dispatch from upstream chart repos
**2. On-Demand (Testing)**: Manual workflow dispatch

#### Smart Detection Logic

The workflow identifies which chart majors are affected by an upstream branch change:

**Example Scenarios**:

| Upstream Branch Changed | Chart Majors Using This Branch | Action |
|------------------------|--------------------------------|--------|
| `dev-v2.15` | v1 only (v2 uses `dev-v2.16`) | Build v1 only |
| `dev-v2.16` | v2 only (v1 uses `dev-v2.15`) | Build v2 only |
| `main` (partner-charts) | v1 and v2 (both use `main` for partner) | Build v1 and v2 |
| `release-v2.15` | v1 only (PROD builds) | Build v1 with prod branches |
| `unknown-branch` | None | Skip build, no match |

**Detection Algorithm**:
1. Receive upstream branch name from webhook or manual input
2. Check ALL chart majors in config.yaml
3. For each chart major, check if ANY of its configured branches (dev or prod) match the upstream branch
4. Build a JSON array of affected chart majors
5. Use matrix strategy to build only those majors

#### Workflow Implementation

**Workflow**: `.github/workflows/head-build.yml`

```yaml
name: Head Build

on:
  # Automated: upstream repos send notification via repository_dispatch
  repository_dispatch:
    types: [upstream-change]
  
  # Manual: for testing/debugging
  workflow_dispatch:
    inputs:
      upstream_repo:
        description: 'Upstream repo that changed (charts, partner, rke2)'
        required: true
        type: choice
        options:
          - charts
          - partner
          - rke2
      upstream_branch:
        description: 'Upstream branch that changed'
        required: true
        type: string
      build_all:
        description: 'Force build all chart majors (ignore smart detection)'
        required: false
        type: boolean
        default: false

permissions:
  contents: write
  packages: write

jobs:
  detect-affected-majors:
    runs-on: ubuntu-latest
    outputs:
      affected_majors: ${{ steps.detect.outputs.affected_majors }}
      has_changes: ${{ steps.detect.outputs.has_changes }}
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Detect affected chart majors
        id: detect
        run: |
          # Get upstream details from webhook or manual input
          if [ "${{ github.event_name }}" = "repository_dispatch" ]; then
            UPSTREAM_REPO="${{ github.event.client_payload.repo }}"
            UPSTREAM_BRANCH="${{ github.event.client_payload.branch }}"
            FORCE_ALL="false"
          else
            UPSTREAM_REPO="${{ github.event.inputs.upstream_repo }}"
            UPSTREAM_BRANCH="${{ github.event.inputs.upstream_branch }}"
            FORCE_ALL="${{ github.event.inputs.build_all }}"
          fi
          
          echo "Upstream change detected: $UPSTREAM_REPO @ $UPSTREAM_BRANCH"
          
          if [ "$FORCE_ALL" = "true" ]; then
            echo "Force build all - skipping smart detection"
            MAJORS=$(yq eval '.chart-versions | keys | .[]' config.yaml | jq -R -s -c 'split("\n")[:-1]')
            echo "affected_majors=$MAJORS" >> $GITHUB_OUTPUT
            echo "has_changes=true" >> $GITHUB_OUTPUT
            exit 0
          fi
          
          # Map repo name to config field
          case "$UPSTREAM_REPO" in
            charts)
              FIELD_SUFFIX="-branch"
              FIELD_NAME="charts"
              ;;
            partner)
              FIELD_SUFFIX="-branch"
              FIELD_NAME="partner"
              ;;
            rke2)
              FIELD_SUFFIX="-branch"
              FIELD_NAME="rke2"
              ;;
            *)
              echo "ERROR: Unknown upstream repo: $UPSTREAM_REPO"
              exit 1
              ;;
          esac
          
          echo "Searching for chart majors using ${FIELD_NAME}-branch = $UPSTREAM_BRANCH"
          
          # Find all chart majors that use this upstream branch in EITHER dev OR prod config
          AFFECTED_MAJORS_RAW=$(yq eval ".chart-versions | to_entries | .[] | select(
            .value.dev.${FIELD_NAME}${FIELD_SUFFIX} == \"${UPSTREAM_BRANCH}\" or 
            .value.prod.${FIELD_NAME}${FIELD_SUFFIX} == \"${UPSTREAM_BRANCH}\"
          ) | .key" config.yaml)
          
          if [ -z "$AFFECTED_MAJORS_RAW" ]; then
            echo "No chart majors found using upstream branch: $UPSTREAM_BRANCH"
            echo "affected_majors=[]" >> $GITHUB_OUTPUT
            echo "has_changes=false" >> $GITHUB_OUTPUT
            exit 0
          fi
          
          # Convert to JSON array for matrix
          AFFECTED_MAJORS=$(echo "$AFFECTED_MAJORS_RAW" | jq -R -s -c 'split("\n")[:-1]')
          
          echo "Affected chart majors: $AFFECTED_MAJORS"
          echo "affected_majors=$AFFECTED_MAJORS" >> $GITHUB_OUTPUT
          echo "has_changes=true" >> $GITHUB_OUTPUT
          
          # Log details for each affected major
          echo "$AFFECTED_MAJORS_RAW" | while read -r MAJOR; do
            DEV_BRANCH=$(yq eval ".chart-versions.\"${MAJOR}\".dev.${FIELD_NAME}${FIELD_SUFFIX}" config.yaml)
            PROD_BRANCH=$(yq eval ".chart-versions.\"${MAJOR}\".prod.${FIELD_NAME}${FIELD_SUFFIX}" config.yaml)
            RANCHER_BRANCH=$(yq eval ".chart-versions.\"${MAJOR}\".rancher-branch" config.yaml)
            echo "  $MAJOR (targets $RANCHER_BRANCH):"
            echo "    - dev.${FIELD_NAME}-branch: $DEV_BRANCH"
            echo "    - prod.${FIELD_NAME}-branch: $PROD_BRANCH"
          done

  build-head-images:
    needs: detect-affected-majors
    if: needs.detect-affected-majors.outputs.has_changes == 'true'
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        major: ${{ fromJson(needs.detect-affected-majors.outputs.affected_majors) }}
        platforms:
          - linux/amd64,linux/arm64
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Setup Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}
      
      - name: Determine head tag
        id: tag
        run: |
          MAJOR="${{ matrix.major }}"
          
          # Get latest stable version for this major
          LATEST=$(yq eval ".chart-versions.\"${MAJOR}\".latest-stable" lock.yaml)
          
          if [ "$LATEST" = "null" ] || [ -z "$LATEST" ]; then
            echo "WARN: No latest-stable in lock.yaml for $MAJOR, querying git tags"
            LATEST=$(git tag -l "${MAJOR}.*" --sort=-version:refname | grep -v '-' | head -1)
          fi
          
          if [ -z "$LATEST" ]; then
            echo "WARN: No git tags found for $MAJOR, using default ${MAJOR}.0.0"
            LATEST="${MAJOR}.0.0"
          fi
          
          # Build dev tag: v1.0.0-dev+abc123
          TAG="${LATEST}-dev+${GITHUB_SHA:0:7}"
          
          echo "Building head image for $MAJOR: $TAG (based on $LATEST)"
          echo "tag=$TAG" >> $GITHUB_OUTPUT
      
      - name: Build and push head image
        run: |
          make push \
            CHART_MAJOR=${{ matrix.major }} \
            VERSION=${{ steps.tag.outputs.tag }} \
            TARGET_PLATFORMS=${{ matrix.platforms }}
  
  update-lock-file:
    needs: [detect-affected-majors, build-head-images]
    if: success() && needs.detect-affected-majors.outputs.has_changes == 'true'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Update upstream refs in lock.yaml
        run: |
          # Get upstream details
          if [ "${{ github.event_name }}" = "repository_dispatch" ]; then
            UPSTREAM_REPO="${{ github.event.client_payload.repo }}"
            UPSTREAM_BRANCH="${{ github.event.client_payload.branch }}"
            UPSTREAM_COMMIT="${{ github.event.client_payload.commit }}"
          else
            UPSTREAM_REPO="${{ github.event.inputs.upstream_repo }}"
            UPSTREAM_BRANCH="${{ github.event.inputs.upstream_branch }}"
            # For manual triggers, fetch latest commit from upstream
            UPSTREAM_COMMIT=$(git ls-remote https://git.rancher.io/${UPSTREAM_REPO} refs/heads/${UPSTREAM_BRANCH} | cut -f1)
          fi
          
          TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
          
          # Update upstream-refs for each affected chart major
          AFFECTED_MAJORS='${{ needs.detect-affected-majors.outputs.affected_majors }}'
          echo "$AFFECTED_MAJORS" | jq -r '.[]' | while read -r MAJOR; do
            echo "Updating lock.yaml upstream-refs for $MAJOR"
            
            yq eval ".chart-versions.\"${MAJOR}\".upstream-refs.${UPSTREAM_REPO}.branch = \"${UPSTREAM_BRANCH}\"" -i lock.yaml
            yq eval ".chart-versions.\"${MAJOR}\".upstream-refs.${UPSTREAM_REPO}.commit = \"${UPSTREAM_COMMIT}\"" -i lock.yaml
            yq eval ".chart-versions.\"${MAJOR}\".upstream-refs.${UPSTREAM_REPO}.fetched-at = \"${TIMESTAMP}\"" -i lock.yaml
          done
          
          yq eval ".generated-at = \"${TIMESTAMP}\"" -i lock.yaml
      
      - name: Commit and push lock file
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          
          if git diff --quiet lock.yaml; then
            echo "No changes to lock.yaml"
            exit 0
          fi
          
          git add lock.yaml
          git commit -m "chore: update lock.yaml after head build

Upstream change: ${{ github.event.client_payload.repo || github.event.inputs.upstream_repo }} @ ${{ github.event.client_payload.branch || github.event.inputs.upstream_branch }}
Affected majors: ${{ needs.detect-affected-majors.outputs.affected_majors }}"
          git push
  
  create-rancher-pr:
    needs: [detect-affected-majors, build-head-images]
    if: success() && needs.detect-affected-majors.outputs.has_changes == 'true'
    runs-on: ubuntu-latest
    strategy:
      matrix:
        major: ${{ fromJson(needs.detect-affected-majors.outputs.affected_majors) }}
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Get head tag for this major
        id: tag
        run: |
          MAJOR="${{ matrix.major }}"
          LATEST=$(yq eval ".chart-versions.\"${MAJOR}\".latest-stable" lock.yaml)
          if [ "$LATEST" = "null" ] || [ -z "$LATEST" ]; then
            LATEST=$(git tag -l "${MAJOR}.*" --sort=-version:refname | grep -v '-' | head -1)
          fi
          if [ -z "$LATEST" ]; then
            LATEST="${MAJOR}.0.0"
          fi
          TAG="${LATEST}-dev+${GITHUB_SHA:0:7}"
          echo "tag=$TAG" >> $GITHUB_OUTPUT
      
      - name: Get Rancher target branch
        id: rancher
        run: |
          MAJOR="${{ matrix.major }}"
          RANCHER_BRANCH=$(yq eval ".chart-versions.\"${MAJOR}\".rancher-branch" config.yaml)
          echo "branch=$RANCHER_BRANCH" >> $GITHUB_OUTPUT
      
      - name: Create PR to rancher/rancher
        run: |
          # Check if PR already exists for this head tag
          EXISTING_PR=$(gh pr list --repo rancher/rancher \
            --base ${{ steps.rancher.outputs.branch }} \
            --search "rancher-charts ${{ steps.tag.outputs.tag }}" \
            --json number --jq '.[0].number' || echo "")
          
          if [ -n "$EXISTING_PR" ]; then
            echo "PR already exists: #$EXISTING_PR"
            exit 0
          fi
          
          gh pr create --repo rancher/rancher \
            --base ${{ steps.rancher.outputs.branch }} \
            --title "Update to rancher-charts ${{ steps.tag.outputs.tag }} (head build)" \
            --body "Automated head build PR

**Chart Major**: ${{ matrix.major }}
**Image Tag**: \`rancher/rancher-charts:${{ steps.tag.outputs.tag }}\`
**Upstream Change**: ${{ github.event.client_payload.repo || github.event.inputs.upstream_repo }} @ ${{ github.event.client_payload.branch || github.event.inputs.upstream_branch }}

This is a HEAD build triggered by upstream changes. Test before merging."
        env:
          GITHUB_TOKEN: ${{ secrets.RANCHER_REPO_TOKEN }}
```

#### Smart Detection Examples

**Scenario 1: Single Chart Major Affected**

```bash
# rancher/charts merges to dev-v2.15
# Config shows only v1 uses dev-v2.15 (v2 uses dev-v2.16)

Trigger: repository_dispatch
  repo: charts
  branch: dev-v2.15

Detection:
  - Check v1: dev.charts-branch = dev-v2.15 ✅ MATCH
  - Check v2: dev.charts-branch = dev-v2.16 ❌ NO MATCH

Result: Build v1 only
  - rancher/rancher-charts:v1.0.0-dev+abc123
  - PR to rancher/rancher release/v2.15
```

**Scenario 2: Multiple Chart Majors Affected**

```bash
# rancher/partner-charts merges to main
# Config shows v1 and v2 BOTH use main for partner-charts

Trigger: repository_dispatch
  repo: partner
  branch: main

Detection:
  - Check v1: dev.partner-branch = main ✅ MATCH
  - Check v2: dev.partner-branch = main ✅ MATCH

Result: Build v1 AND v2 (parallel)
  - rancher/rancher-charts:v1.0.0-dev+abc123
  - rancher/rancher-charts:v2.0.0-dev+abc123
  - PR to rancher/rancher release/v2.15 (for v1)
  - PR to rancher/rancher release/v2.16 (for v2)
```

**Scenario 3: Production Branch Change**

```bash
# rancher/charts merges to release-v2.15 (stable branch)
# Config shows v1 uses release-v2.15 for prod builds

Trigger: repository_dispatch
  repo: charts
  branch: release-v2.15

Detection:
  - Check v1: prod.charts-branch = release-v2.15 ✅ MATCH
  - Check v2: prod.charts-branch = release-v2.16 ❌ NO MATCH

Result: Build v1 only (with PROD branches)
  - rancher/rancher-charts:v1.0.0-dev+abc123
  - Note: Still tagged as dev build (not a release)
  - PR to rancher/rancher release/v2.15
```

**Scenario 4: No Match**

```bash
# rancher/charts merges to experimental-branch
# Config shows no chart majors use this branch

Trigger: repository_dispatch
  repo: charts
  branch: experimental-branch

Detection:
  - Check v1: No match
  - Check v2: No match

Result: Skip build
  - No images built
  - Job exits cleanly
```

#### Upstream Repository Integration

To enable automated head builds, upstream chart repositories need to send notifications when branches change.

**Add to rancher/charts, rancher/partner-charts, rancher/rke2-charts**:

**File**: `.github/workflows/notify-rancher-assets.yml`

```yaml
name: Notify rancher-assets on branch changes

on:
  push:
    branches:
      # Development branches
      - 'dev-v*'
      - 'main'
      # Production/stable branches
      - 'release-v*'
      - 'main-source'

jobs:
  notify:
    runs-on: ubuntu-latest
    steps:
      - name: Determine repo name
        id: repo
        run: |
          # Extract repo name from GITHUB_REPOSITORY (e.g., rancher/charts → charts)
          REPO_NAME=$(echo ${{ github.repository }} | cut -d/ -f2)
          echo "name=$REPO_NAME" >> $GITHUB_OUTPUT
      
      - name: Send repository dispatch to rancher-assets
        run: |
          curl -X POST \
            -H "Accept: application/vnd.github+json" \
            -H "Authorization: Bearer ${{ secrets.RANCHER_ASSETS_DISPATCH_TOKEN }}" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            https://api.github.com/repos/rancher/rancher-assets/dispatches \
            -d '{
              "event_type": "upstream-change",
              "client_payload": {
                "repo": "${{ steps.repo.outputs.name }}",
                "branch": "${{ github.ref_name }}",
                "commit": "${{ github.sha }}",
                "pusher": "${{ github.actor }}"
              }
            }'
```

**Required Secret**: `RANCHER_ASSETS_DISPATCH_TOKEN`
- Create PAT with `repo` scope
- Add to each upstream repo as secret
- Allows sending repository_dispatch events to rancher-assets

#### Manual Testing

Developers can trigger head builds manually for testing:

```bash
# Via GitHub UI: Actions → Head Build → Run workflow
# Select:
#   - upstream_repo: charts
#   - upstream_branch: dev-v2.15
#   - build_all: false (use smart detection)

# Result: Builds only v1 (if v1 uses dev-v2.15)
```

**Force build all (bypass smart detection)**:

```bash
# Via GitHub UI: Actions → Head Build → Run workflow
# Select:
#   - build_all: true

# Result: Builds ALL chart majors regardless of branch match
```

#### Benefits of Smart Detection

✅ **Resource Efficient**: Only builds what changed, not everything  
✅ **Faster Feedback**: PRs arrive only for affected Rancher branches  
✅ **Clear Traceability**: Lock file tracks which upstream commit is bundled  
✅ **Parallel Builds**: Multiple affected majors build concurrently  
✅ **Safe Defaults**: Unknown branches skip build rather than building everything  
✅ **Manual Override**: Force build all when needed for testing  

---

## Integration with Rancher

### How Rancher Knows Which Version to Use

**Current mechanism** (preserved):
- `rancher/rancher` repo has `build.yaml`: `defaultChartsImage: rancher/rancher-charts:v1.0.0`
- `go generate` updates `chart/values.yaml` from `build.yaml`
- Rancher Helm chart uses version from values
- Users can override: `--set chartsImage.tag=vX.Y.Z`

**Auto-PR updates `build.yaml`**:
1. Charts image released (e.g., v1.0.0)
2. Auto-PR to rancher/rancher updates:
   ```yaml
   # build.yaml
   defaultChartsImage: rancher/rancher-charts:v1.0.0
   ```
3. Developer runs `go generate`
4. `chart/values.yaml` updated automatically
5. PR merged, Rancher picks up new charts version

**Version Resolution Flow**:
```
rancher-assets tag v1.0.0
  → Build charts image
  → Auto-PR to rancher/rancher release/v2.15
  → Updates build.yaml: defaultChartsImage: v1.0.0
  → Developer runs: go generate
  → Updates chart/values.yaml: chartsImage.tag: v1.0.0
  → Rancher release includes v1.0.0
```

---

## Migration Path

### Phase 1: Create Repository

1. **Create `rancher/rancher-assets` repo**
2. **Port files from rancher/rancher**:
   - `package/Dockerfile-charts` → initial template
   - `.github/workflows/release-charts-image.yml` → reference
   - `.github/workflows/charts-update-pr.yml` → reference
   - Chart branch configs from `build.yaml` → `config.yaml`

### Phase 2: Build Generator

1. **Create Go generator**:
   - Parse `config.yaml`
   - Generate Dockerfiles per chart major
   - Update `lock.yaml`
2. **Create Makefile**
3. **Test locally**: `make generate && make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1`

### Phase 3: Setup CI

1. **Add workflows**:
   - `pull-request.yml` (validate PRs run `make generate` and `make verify`)
   - `release.yml` (tag-based builds)
   - `head-build.yml` (upstream changes)
2. **Test with RC tags**: `v1.0.0-rc.1`

### Phase 4: Production Release

1. **Tag first stable release**: `v1.0.0`
2. **Verify auto-PR to rancher/rancher**
3. **Merge PR, validate Rancher uses new version**

### Phase 5: Upstream Webhooks

1. **Configure repository_dispatch webhooks** on chart repos
2. **Test head builds** on upstream merges

---

## Appendix: Alternatives Considered

### Alternative 1: Traditional Release Branches

**Structure**: `main`, `release/v1`, `release/v2` branches

**Pros**:
- Familiar Git workflow
- Version-specific Dockerfiles possible
- Easy to see "what's in v1?"

**Cons**:
- Backporting overhead (every bug fix to N branches)
- Branch proliferation (one per chart major)
- Workflow drift risk (branches diverge)
- Main branch confusion (what is it for?)
- Head build complexity (which branch to build from?)

**Decision**: Rejected - backport overhead outweighs benefits

---

### Alternative 2: Keep in rancher/rancher Repo

**Pros**:
- No new repo needed
- Charts build alongside Rancher

**Cons**:
- Chicken-and-egg: building asset consumed by same repo
- Upstream repos not equal citizens
- Head builds harder (rancher/rancher not the "owner")
- Coupling between chart releases and Rancher development

**Decision**: Rejected - architectural coupling too tight

---

### Alternative 3: Manual Dockerfiles (No Generator)

**Pros**:
- Simple, direct
- No generator code to maintain

**Cons**:
- Manual updates needed per chart major
- Duplication across Dockerfile variants
- Easy to drift or make mistakes
- Harder to audit what changed

**Decision**: Rejected - generator provides consistency and auditability

---

## Summary

This design provides:

✅ **Mono-branch simplicity**: Single `main` branch, no backports  
✅ **Generator-based consistency**: Dockerfiles generated, not hand-written  
✅ **Independent versioning**: Charts SemVer independent from Rancher  
✅ **Smart build detection**: Tag format determines prod vs dev  
✅ **Automatic integration**: Auto-PR to rancher/rancher  
✅ **Head build support**: Upstream changes trigger rebuilds  
✅ **Auditability**: Lock file tracks what's bundled  
✅ **Local development**: `make generate` works anywhere  

**Next Steps**:
1. Create rancher-assets repository
2. Implement Go generator (main.go + internal/)
3. Add initial config.yaml (v1 for Rancher 2.15)
4. Test with RC tags
5. Configure upstream webhooks
6. Production release v1.0.0
