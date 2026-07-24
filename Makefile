# Include logic that can be reused across projects.
include hack/make/build.mk

# ---- CI Image Config ----
CI_IMAGE := ghcr.io/rancher/ci-image/go1.26
WORKDIR := /workspace

# Detect CI environment (common env var used by many CI systems)
CI ?= false

# Docker run wrapper (only used locally)
DOCKER_RUN = docker run --rm -i \
	-v $(PWD):$(WORKDIR) \
	-w $(WORKDIR) \
	$(CI_IMAGE)

# Command runner:
# - In CI: run commands directly
# - Locally: run via Docker
ifeq ($(CI),true)
	RUN =
else
	RUN = $(DOCKER_RUN)
endif

# ---- Build Config ----
# Define target platforms, image builder and the fully qualified image name.
TARGET_PLATFORMS ?= linux/amd64,linux/arm64

REPO ?= rancher
IMAGE ?= rancher-assets
IMAGE_NAME = $(REPO)/$(IMAGE)
FULL_IMAGE_TAG = $(IMAGE_NAME):$(TAG)
BUILD_ACTION ?= --load

# TAG is the version (set by CI or required for local builds)
TAG ?=
RANCHER_MINOR ?=

# DEV flag determines Dockerfile variant
DEV ?= false
DOCKERFILE ?= Dockerfile.$(RANCHER_MINOR)
ifeq ($(DEV),true)
	DOCKERFILE := $(DOCKERFILE)-dev
endif

# Build arguments - provided by CI env vars or computed at build time
GIT_COMMIT ?= $(shell ./scripts/version --short COMMIT)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_URL ?= https://github.com/rancher/rancher-assets
TARGET_BRANCH ?= $(shell ./scripts/version --short TARGET_BRANCH)

TARGETS := $(shell ls scripts|grep -ve "^util-\|entry\|^pull-scripts")

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)
$(TARGETS):
	$(RUN) ./scripts/$@

build-image: buildx-machine ## build (and load) the container image targeting the current platform.
	$(IMAGE_BUILDER) build -f dockerfiles/$(DOCKERFILE) \
		--builder $(MACHINE) $(IMAGE_ARGS) \
		--build-arg VERSION=$(TAG) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg TARGET_BRANCH=$(TARGET_BRANCH) \
		--build-arg BUILD_URL=$(BUILD_URL) \
		--platform=$(TARGET_PLATFORMS) \
		-t "$(FULL_IMAGE_TAG)" \
		$(BUILD_ACTION) .
	@echo "Built $(FULL_IMAGE_TAG)"

build-validate: buildx-machine ## build (and load) the container image targeting the current platform.
	mkdir -p ci
	$(IMAGE_BUILDER) build -f dockerfiles/$(DOCKERFILE) \
		--builder $(MACHINE) $(IMAGE_ARGS) \
		--build-arg VERSION=$(TAG) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg TARGET_BRANCH=$(TARGET_BRANCH) \
		--build-arg BUILD_URL=$(BUILD_URL) \
		--platform=$(TARGET_PLATFORMS) \
		--output type=oci,dest=ci/multiarch-image.oci \
		-t "$(FULL_IMAGE_TAG)" .
	@echo "Built $(FULL_IMAGE_TAG) multi-arch image saved to ci/multiarch-image.oci"

push-image: validate buildx-machine ## build the container image targeting all platforms defined by TARGET_PLATFORMS and push to a registry.
	$(IMAGE_BUILDER) build -f dockerfiles/$(DOCKERFILE) \
		--builder $(MACHINE) $(IMAGE_ARGS) $(IID_FILE_FLAG) $(BUILDX_ARGS) \
		--build-arg VERSION=$(TAG) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg TARGET_BRANCH=$(TARGET_BRANCH) \
		--build-arg BUILD_URL=$(BUILD_URL) \
		--platform=$(TARGET_PLATFORMS) \
		-t "$(FULL_IMAGE_TAG)" \
		--push .
	@echo "Pushed $(FULL_IMAGE_TAG)"

.PHONY: generate
generate: ## Run go generate to update generated code.
	@echo "Generating Dockerfiles and updating lock.yaml..."
	$(RUN) go run main.go generate

.PHONY: vendor-update
vendor-update: ## Update Go dependencies and vendor them
	@./scripts/vendor-update.sh

.PHONY: validate
validate: validate-dirty ## Run validation checks.

.PHONY: validate-dirty
validate-dirty:
ifdef DIRTY
	@echo Git is dirty
	@git --no-pager status
	@git --no-pager diff
	@exit 1
endif

.PHONY: verify
verify: ## Verify no uncommitted changes in generated files
	@echo "Verifying generated files are committed..."
	@if [ -n "$$(git status --porcelain dockerfiles/ lock.yaml)" ]; then \
		echo "❌ Error: uncommitted changes detected in generated files"; \
		echo ""; \
		git status --porcelain dockerfiles/ lock.yaml; \
		echo ""; \
		echo "Run 'make generate' and commit the changes"; \
		exit 1; \
	fi
	@echo "✅ Verified: all generated files are committed"

.PHONY: test
test: ## Run tests
	$(RUN) go test -v ./...

.PHONY: export-images
export-images: ## Generate image lists from chart catalogs (requires TAG and RANCHER_MINOR, optional: LOCAL=true for local builds)
	@if [ -z "$(TAG)" ] || [ -z "$(RANCHER_MINOR)" ]; then \
		echo "❌ Error: TAG and RANCHER_MINOR required"; \
		echo "Usage: make export-images TAG=v2.15-20260716T1430Z RANCHER_MINOR=2.15 [LOCAL=true]"; \
		exit 1; \
	fi
	@LOCAL_FLAG=""; \
	if [ "$(LOCAL)" = "true" ]; then \
		LOCAL_FLAG="--local"; \
	fi; \
	./scripts/export-image-lists.sh \
		--image "$(IMAGE_NAME):$(TAG)" \
		--version "$(TAG)" \
		--output-dir "dist/$(TAG)" \
		$$LOCAL_FLAG

.PHONY: export-images-debug
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
		echo "  4. cd /tmp/charts/v2 && for dir in */; do (cd \"\$$\$$dir\" && git config --local core.bare false && git checkout -- .); done"; \
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
	$(RUN) go run main.go export-images \
		--charts-path "$(CHARTS_PATH)" \
		--version "$$VERSION" \
		--output-dir "$$OUTPUT_DIR"