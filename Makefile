SHELL := /bin/bash
.DEFAULT_GOAL := help

DOCKERFILES_DIR := dockerfiles
VERSION ?=
CHART_MAJOR ?=
PUSH ?= false

# Fork-friendly configuration - override these for your fork
REGISTRY ?= ghcr.io
ORG ?= rancher
REPO ?= rancher-assets
SOURCE_REPO ?= rancher/rancher-assets
IMAGE_REPO ?= $(REGISTRY)/$(ORG)/$(REPO)
TARGET_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help generate verify export-images build build-all build-release build-release-with-lists push-image push-all vendor-update release-auto release-manual

help: ## Show this help message
	@echo "Rancher Assets Build System"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make generate"
	@echo "  make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1"
	@echo "  make build-all                    # Dev builds with auto-generated versions"
	@echo "  make push-all                     # Build and push all to registry"
	@echo "  make build-release                # Local debug - builds latest-stable from lock.yaml"
	@echo "  make build-release-with-lists     # Local debug - builds + generates image lists"
	@echo "  make export-images CHART_MAJOR=v1 VERSION=v1.0.0  # Generate image lists"
	@echo "  make release-auto                 # Create auto pre-release tags"
	@echo "  make release-manual BUMP=minor RELEASE=prerelease"
	@echo ""
	@echo "Fork-friendly overrides:"
	@echo "  make push-all REGISTRY=ghcr.io ORG=myorg REPO=my-charts SOURCE_REPO=myorg/rancher-assets"
	@echo "  Or set in environment: export REGISTRY=ghcr.io ORG=myorg"

generate: ## Generate Dockerfiles from config.yaml
	@echo "Generating Dockerfiles and updating lock.yaml..."
	@go run main.go generate

verify: ## Verify no uncommitted changes in generated files
	@echo "Verifying generated files are committed..."
	@if [ -n "$$(git status --porcelain $(DOCKERFILES_DIR) lock.yaml)" ]; then \
		echo "❌ Error: uncommitted changes detected in generated files"; \
		echo ""; \
		git status --porcelain $(DOCKERFILES_DIR) lock.yaml; \
		echo ""; \
		echo "Run 'make generate' and commit the changes"; \
		exit 1; \
	fi
	@echo "✅ Verified: all generated files are committed"

export-images: ## Generate image lists from chart catalogs (requires CHART_MAJOR and VERSION, optional: LOCAL=true for local builds)
	@if [ -z "$(CHART_MAJOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: CHART_MAJOR and VERSION required"; \
		echo "Usage: make export-images CHART_MAJOR=v1 VERSION=v1.0.0 [LOCAL=true]"; \
		exit 1; \
	fi
	@LOCAL_FLAG=""; \
	if [ "$(LOCAL)" = "true" ]; then \
		LOCAL_FLAG="--local"; \
	fi; \
	./scripts/export-image-lists.sh \
		--image "$(IMAGE_REPO):$(VERSION)" \
		--version "$(VERSION)" \
		--output-dir "dist/$(VERSION)" \
		$$LOCAL_FLAG

vendor-update: ## Update Go dependencies and vendor them
	@./scripts/vendor-update.sh

build: ## Build chart image (requires CHART_MAJOR and VERSION)
	@if [ -z "$(CHART_MAJOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: CHART_MAJOR and VERSION required"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1  # Dev build"; \
		echo "  make build CHART_MAJOR=v1 VERSION=v1.0.0       # Prod build"; \
		exit 1; \
	fi
	@if [ ! -f "$(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)" ]; then \
		echo "❌ Error: Dockerfile not found: $(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)"; \
		echo "Run 'make generate' first"; \
		exit 1; \
	fi
	@eval $$(./scripts/get-build-vars.sh --major $(CHART_MAJOR) --version $(VERSION) --format shell); \
	echo "Building $(CHART_MAJOR) version $(VERSION) ($$BUILD_TYPE)"; \
	echo ""; \
	echo "Configuration:"; \
	echo "  Build type: $$BUILD_TYPE"; \
	echo "  Charts branch: $$CHART_BRANCH (commit: $${CHART_COMMIT:0:8})"; \
	echo "  Partner branch: $$PARTNER_BRANCH (commit: $${PARTNER_COMMIT:0:8})"; \
	echo "  RKE2 branch: $$RKE2_BRANCH (commit: $${RKE2_COMMIT:0:8})"; \
	echo "  Rancher branch: $$RANCHER_BRANCH"; \
	echo ""; \
	echo "Building image..."; \
	docker buildx build \
		--file "$(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)" \
		--platform "$(TARGET_PLATFORMS)" \
		--build-arg BUILD_TYPE=$$BUILD_TYPE \
		--build-arg CHART_BRANCH=$$CHART_BRANCH \
		--build-arg PARTNER_BRANCH=$$PARTNER_BRANCH \
		--build-arg RKE2_BRANCH=$$RKE2_BRANCH \
		--build-arg CHART_COMMIT=$$CHART_COMMIT \
		--build-arg PARTNER_COMMIT=$$PARTNER_COMMIT \
		--build-arg RKE2_COMMIT=$$RKE2_COMMIT \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$$(git rev-parse HEAD) \
		--build-arg BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		--build-arg TARGET_BRANCH=$$RANCHER_BRANCH \
		--build-arg BUILD_URL="https://github.com/$(SOURCE_REPO)" \
		--tag "$(IMAGE_REPO):$(VERSION)" \
		--load \
		. && \
	echo "" && \
	echo "✅ Build complete: $(IMAGE_REPO):$(VERSION)"

push-image: ## Push image (for use with ecm-distro-tools)
	@if [ -z "$(CHART_MAJOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: CHART_MAJOR and VERSION required"; \
		exit 1; \
	fi
	@eval $$(./scripts/get-build-vars.sh --major $(CHART_MAJOR) --version $(VERSION) --format shell); \
	docker buildx build \
		--file "$(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)" \
		--platform "$(TARGET_PLATFORMS)" \
		--build-arg BUILD_TYPE=$$BUILD_TYPE \
		--build-arg CHART_BRANCH=$$CHART_BRANCH \
		--build-arg PARTNER_BRANCH=$$PARTNER_BRANCH \
		--build-arg RKE2_BRANCH=$$RKE2_BRANCH \
		--build-arg CHART_COMMIT=$$CHART_COMMIT \
		--build-arg PARTNER_COMMIT=$$PARTNER_COMMIT \
		--build-arg RKE2_COMMIT=$$RKE2_COMMIT \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg TARGET_BRANCH=$$RANCHER_BRANCH \
		--build-arg BUILD_URL=$(BUILD_URL) \
		--tag "$(IMAGE_REPO):$(VERSION)" \
		--push \
		.

build-all: ## Build all chart versions from lock.yaml with auto-generated versions
	@echo "Building all chart versions from lock.yaml"
	@echo ""
	@CHART_MAJORS=$$(yq eval '.chart-versions | keys | .[]' lock.yaml); \
	if [ -z "$$CHART_MAJORS" ]; then \
		echo "❌ Error: No chart versions found in lock.yaml"; \
		exit 1; \
	fi; \
	BUILD_DATE=$$(date -u +%Y%m%d); \
	GIT_SHORT=$$(git rev-parse --short HEAD); \
	for major in $$CHART_MAJORS; do \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "Building $$major"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		MAJOR_NUM=$${major#v}; \
		VERSION="$$major.0.0-dev.$$BUILD_DATE.$$GIT_SHORT"; \
		echo "Auto-generated version: $$VERSION"; \
		$(MAKE) build CHART_MAJOR=$$major VERSION=$$VERSION; \
		if [ $$? -ne 0 ]; then \
			echo "❌ Build failed for $$major"; \
			exit 1; \
		fi; \
		echo ""; \
	done; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	echo "✅ All builds complete"; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

push-all: ## Build and push all chart versions to registry
	@echo "⚠️  WARNING: This will push images to $(IMAGE_REPO)"
	@echo "⚠️  Make sure you are authenticated to $(REGISTRY)"
	@echo ""
	@read -p "Continue? [y/N] " -n 1 -r; \
	echo ""; \
	if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "Aborted."; \
		exit 1; \
	fi
	@echo ""
	@echo "Building and pushing all chart versions from lock.yaml"
	@echo ""
	@CHART_MAJORS=$$(yq eval '.chart-versions | keys | .[]' lock.yaml); \
	if [ -z "$$CHART_MAJORS" ]; then \
		echo "❌ Error: No chart versions found in lock.yaml"; \
		exit 1; \
	fi; \
	BUILD_DATE=$$(date -u +%Y%m%d); \
	GIT_SHORT=$$(git rev-parse --short HEAD); \
	for major in $$CHART_MAJORS; do \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "Building and pushing $$major"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		MAJOR_NUM=$${major#v}; \
		VERSION="$$major.0.0-dev.$$BUILD_DATE.$$GIT_SHORT"; \
		echo "Version: $$VERSION"; \
		echo "Target: $(IMAGE_REPO):$$VERSION"; \
		echo ""; \
		\
		RANCHER_BRANCH=$$(yq eval ".chart-versions.\"$$major\".rancher-branch" config.yaml 2>/dev/null); \
		CHART_BRANCH=$$(yq eval ".chart-versions.\"$$major\".upstream-refs.dev.charts.branch" lock.yaml 2>/dev/null); \
		PARTNER_BRANCH=$$(yq eval ".chart-versions.\"$$major\".upstream-refs.dev.partner.branch" lock.yaml 2>/dev/null); \
		RKE2_BRANCH=$$(yq eval ".chart-versions.\"$$major\".upstream-refs.dev.rke2.branch" lock.yaml 2>/dev/null); \
		CHART_COMMIT=$$(yq eval ".chart-versions.\"$$major\".upstream-refs.dev.charts.commit" lock.yaml 2>/dev/null); \
		PARTNER_COMMIT=$$(yq eval ".chart-versions.\"$$major\".upstream-refs.dev.partner.commit" lock.yaml 2>/dev/null); \
		RKE2_COMMIT=$$(yq eval ".chart-versions.\"$$major\".upstream-refs.dev.rke2.commit" lock.yaml 2>/dev/null); \
		\
		docker buildx build \
			--file "$(DOCKERFILES_DIR)/Dockerfile.$$major" \
			--platform "$(TARGET_PLATFORMS)" \
			--build-arg BUILD_TYPE=dev \
			--build-arg CHART_BRANCH=$$CHART_BRANCH \
			--build-arg PARTNER_BRANCH=$$PARTNER_BRANCH \
			--build-arg RKE2_BRANCH=$$RKE2_BRANCH \
			--build-arg CHART_COMMIT=$$CHART_COMMIT \
			--build-arg PARTNER_COMMIT=$$PARTNER_COMMIT \
			--build-arg RKE2_COMMIT=$$RKE2_COMMIT \
			--build-arg VERSION=$$VERSION \
			--build-arg GIT_COMMIT=$$(git rev-parse HEAD) \
			--build-arg BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
			--build-arg TARGET_BRANCH=$$RANCHER_BRANCH \
			--build-arg BUILD_URL="https://github.com/$(SOURCE_REPO)" \
			--tag "$(IMAGE_REPO):$$VERSION" \
			--push \
			.; \
		if [ $$? -ne 0 ]; then \
			echo "❌ Push failed for $$major"; \
			exit 1; \
		fi; \
		echo "✅ Pushed: $(IMAGE_REPO):$$VERSION"; \
		echo ""; \
	done; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	echo "✅ All images pushed successfully"; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

build-release: ## Build release versions from lock.yaml (LOCAL DEBUG ONLY - use GHA for real releases)
	@./scripts/build-releases.sh

build-release-with-lists: ## Build releases and generate image lists (LOCAL DEBUG ONLY)
	@./scripts/build-releases.sh --with-lists

.PHONY: test
test: ## Run tests
	@go test -v ./...

release-auto: ## Create auto pre-release tags based on lock.yaml changes
	@./scripts/create-auto-prerelease.sh

release-manual: ## Create manual release tags (usage: make release-manual BUMP=minor RELEASE=prerelease [MAJOR=v0])
	@if [ -z "$(BUMP)" ] || [ -z "$(RELEASE)" ]; then \
		echo "❌ Error: BUMP and RELEASE required"; \
		echo ""; \
		echo "Usage:"; \
		echo "  make release-manual BUMP=minor RELEASE=prerelease"; \
		echo "  make release-manual BUMP=patch RELEASE=stable MAJOR=v0"; \
		exit 1; \
	fi
	@ARGS="--bump=$(BUMP) --release=$(RELEASE)"; \
	if [ -n "$(MAJOR)" ]; then \
		ARGS="$$ARGS --major=$(MAJOR)"; \
	fi; \
	if [ -n "$(COMMIT)" ]; then \
		ARGS="$$ARGS --commit=$(COMMIT)"; \
	fi; \
	./scripts/create-manual-release.sh $$ARGS
