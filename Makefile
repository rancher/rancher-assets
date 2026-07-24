SHELL := /bin/bash
.DEFAULT_GOAL := help

DOCKERFILES_DIR := dockerfiles
VERSION ?=
RANCHER_MINOR ?=
PUSH ?= false

# Fork-friendly configuration - override these for your fork
REGISTRY ?= ghcr.io
ORG ?= rancher
REPO ?= rancher-assets
SOURCE_REPO ?= rancher/rancher-assets
IMAGE_REPO ?= $(REGISTRY)/$(ORG)/$(REPO)
TARGET_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help generate verify export-images export-images-debug build build-all build-release build-release-with-lists push-image push-all vendor-update release-auto release-manual

help: ## Show this help message
	@echo "Rancher Assets Build System"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make generate"
	@echo "  make build RANCHER_MINOR=v1 VERSION=v1.0.0-rc.1"
	@echo "  make build-all                    # Dev builds with auto-generated versions"
	@echo "  make push-all                     # Build and push all to registry"
	@echo "  make build-release                # Local debug - builds latest-stable from lock.yaml"
	@echo "  make build-release-with-lists     # Local debug - builds + generates image lists"
	@echo "  make export-images RANCHER_MINOR=v1 VERSION=v1.0.0  # Generate image lists"
	@echo "  make export-images-debug CHARTS_PATH=/tmp/charts/v2  # Debug image list code"
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

export-images: ## Generate image lists from chart catalogs (requires RANCHER_MINOR and VERSION, optional: LOCAL=true for local builds)
	@if [ -z "$(RANCHER_MINOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: RANCHER_MINOR and VERSION required"; \
		echo "Usage: make export-images RANCHER_MINOR=v1 VERSION=v1.0.0 [LOCAL=true]"; \
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

export-images-debug: ## Debug: Generate image lists from local chart catalogs (requires CHARTS_PATH)
	@if [ -z "$(CHARTS_PATH)" ]; then \
		echo "❌ Error: CHARTS_PATH required"; \
		echo ""; \
		echo "Usage:"; \
		echo "  make export-images-debug CHARTS_PATH=/tmp/rancher-assets-charts-v1.0.0/v2"; \
		echo ""; \
		echo "This target is for debugging the image list generation code."; \
		echo "Point CHARTS_PATH to a directory containing the extracted chart catalogs"; \
		echo "(rancher-charts, rancher-partner-charts, rancher-rke2-charts)."; \
		echo ""; \
		echo "To extract catalogs from an existing image:"; \
		echo "  1. docker create --name tmp-extract IMAGE_NAME"; \
		echo "  2. docker cp tmp-extract:/var/lib/rancher-data/local-catalogs/v2 /tmp/charts"; \
		echo "  3. docker rm tmp-extract"; \
		echo "  4. cd /tmp/charts/v2 && for dir in */; do (cd \"\$$dir\" && git config --local core.bare false && git checkout -- .); done"; \
		echo "  5. make export-images-debug CHARTS_PATH=/tmp/charts/v2"; \
		exit 1; \
	fi
	@VERSION=$${VERSION:-debug}; \
	OUTPUT_DIR=$${OUTPUT_DIR:-dist/debug}; \
	echo "Scanning charts for image references..."; \
	echo "  Charts path: $(CHARTS_PATH)"; \
	echo "  Output dir: $$OUTPUT_DIR"; \
	echo "  Version: $$VERSION"; \
	echo ""; \
	go run main.go export-images \
		--charts-path "$(CHARTS_PATH)" \
		--version "$$VERSION" \
		--output-dir "$$OUTPUT_DIR"

vendor-update: ## Update Go dependencies and vendor them
	@./scripts/vendor-update.sh

build: ## Build chart image (requires RANCHER_MINOR and VERSION)
	@if [ -z "$(RANCHER_MINOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: RANCHER_MINOR and VERSION required"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make build RANCHER_MINOR=2.15 VERSION=v2.15-20260716T1430Z-dev  # Dev build"; \
		echo "  make build RANCHER_MINOR=2.15 VERSION=v2.15-20260805T1600Z       # Prod build"; \
		exit 1; \
	fi
	@# Select Dockerfile based on VERSION suffix (-dev or not)
	@DOCKERFILE=$$(if echo "$(VERSION)" | grep -q -- "-dev$$"; then \
		echo "$(DOCKERFILES_DIR)/Dockerfile.$(RANCHER_MINOR)-dev"; \
	else \
		echo "$(DOCKERFILES_DIR)/Dockerfile.$(RANCHER_MINOR)"; \
	fi); \
	if [ ! -f "$$DOCKERFILE" ]; then \
		echo "❌ Error: Dockerfile not found: $$DOCKERFILE"; \
		echo "Run 'make generate' first"; \
		exit 1; \
	fi; \
	BUILD_TYPE=$$(if echo "$(VERSION)" | grep -q -- "-dev$$"; then echo "dev"; else echo "prod"; fi); \
	echo "Building Rancher $(RANCHER_MINOR) version $(VERSION) ($$BUILD_TYPE)"; \
	echo "  Dockerfile: $$DOCKERFILE"; \
	echo ""; \
	docker buildx build \
		--file "$$DOCKERFILE" \
		--platform "$(TARGET_PLATFORMS)" \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$$(git rev-parse HEAD) \
		--build-arg BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		--build-arg TARGET_BRANCH=$$(yq eval ".chart-versions.\"$(RANCHER_MINOR)\".rancher-branch" config.yaml) \
		--build-arg BUILD_URL="https://github.com/$(SOURCE_REPO)" \
		--tag "$(IMAGE_REPO):$(VERSION)" \
		--load \
		.; \
	echo ""; \
	echo "✅ Build complete: $(IMAGE_REPO):$(VERSION)"

push-image: ## Push image (for use with ecm-distro-tools)
	@if [ -z "$(RANCHER_MINOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: RANCHER_MINOR and VERSION required"; \
		exit 1; \
	fi
	@# Select Dockerfile based on VERSION suffix (-dev or not)
	@DOCKERFILE=$$(if echo "$(VERSION)" | grep -q -- "-dev$$"; then \
		echo "$(DOCKERFILES_DIR)/Dockerfile.$(RANCHER_MINOR)-dev"; \
	else \
		echo "$(DOCKERFILES_DIR)/Dockerfile.$(RANCHER_MINOR)"; \
	fi); \
	if [ ! -f "$$DOCKERFILE" ]; then \
		echo "❌ Error: Dockerfile not found: $$DOCKERFILE"; \
		echo "Run 'make generate' first"; \
		exit 1; \
	fi; \
	eval $$(./scripts/get-build-vars.sh --minor $(RANCHER_MINOR) --version $(VERSION) --format shell); \
	BUILD_TYPE=$$(if echo "$(VERSION)" | grep -q -- "-dev$$"; then echo "dev"; else echo "prod"; fi); \
	echo "Pushing Rancher $(RANCHER_MINOR) version $(VERSION) ($$BUILD_TYPE)"; \
	echo "  Dockerfile: $$DOCKERFILE"; \
	docker buildx build \
		--file "$$DOCKERFILE" \
		--platform "$(TARGET_PLATFORMS)" \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg TARGET_BRANCH=$$RANCHER_BRANCH \
		--build-arg BUILD_URL=$(BUILD_URL) \
		--tag "$(IMAGE_REPO):$(VERSION)" \
		--push \
		.

build-all: ## Build all chart versions from lock.yaml with auto-generated versions
	@echo "Building all Rancher minors with CalVer dev versions"
	@echo ""
	@RANCHER_MINORS=$$(go run main.go list-minors); \
	if [ -z "$$RANCHER_MINORS" ]; then \
		echo "❌ Error: No Rancher minors found in config.yaml"; \
		exit 1; \
	fi; \
	for minor in $$RANCHER_MINORS; do \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "Building Rancher $$minor"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		VERSION=$$(go run main.go calver-dev-version --minor=$$minor); \
		echo "Auto-generated CalVer version: $$VERSION"; \
		$(MAKE) build RANCHER_MINOR=$$minor VERSION=$$VERSION; \
		if [ $$? -ne 0 ]; then \
			echo "❌ Build failed for Rancher $$minor"; \
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
	@RANCHER_MINORS=$$(yq eval '.chart-versions | keys | .[]' lock.yaml); \
	if [ -z "$$RANCHER_MINORS" ]; then \
		echo "❌ Error: No chart versions found in lock.yaml"; \
		exit 1; \
	fi; \
	BUILD_DATE=$$(date -u +%Y%m%d); \
	GIT_SHORT=$$(git rev-parse --short HEAD); \
	for major in $$RANCHER_MINORS; do \
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
		\
		docker buildx build \
			--file "$(DOCKERFILES_DIR)/Dockerfile.$$major-dev" \
			--platform "$(TARGET_PLATFORMS)" \
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

release-manual: ## Create manual release tags (usage: make release-manual RELEASE=prerelease [MINOR=2.15])
	@if [ -z "$(RELEASE)" ]; then \
		echo "❌ Error: RELEASE required"; \
		echo ""; \
		echo "Usage:"; \
		echo "  make release-manual RELEASE=prerelease         # All minors"; \
		echo "  make release-manual RELEASE=stable MINOR=2.15  # Specific minor"; \
		exit 1; \
	fi
	@ARGS="--release=$(RELEASE)"; \
	if [ -n "$(MINOR)" ]; then \
		ARGS="$$ARGS --minor=$(MINOR)"; \
	fi; \
	if [ -n "$(COMMIT)" ]; then \
		ARGS="$$ARGS --commit=$(COMMIT)"; \
	fi; \
	./scripts/create-manual-release.sh $$ARGS
