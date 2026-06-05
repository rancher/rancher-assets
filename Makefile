SHELL := /bin/bash
.DEFAULT_GOAL := help

DOCKERFILES_DIR := dockerfiles
VERSION ?=
CHART_MAJOR ?=
ORG ?= rancher
REPO ?= rancher-charts
IMAGE_REPO ?= docker.io/$(ORG)/$(REPO)
TARGET_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: help generate verify build build-all build-release

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
	@echo "  make build-all              # Dev builds with auto-generated versions"
	@echo "  make build-release          # Local debug - builds latest-stable from lock.yaml"

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

build: ## Build chart image (requires CHART_MAJOR and VERSION)
	@if [ -z "$(CHART_MAJOR)" ] || [ -z "$(VERSION)" ]; then \
		echo "❌ Error: CHART_MAJOR and VERSION required"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make build CHART_MAJOR=v1 VERSION=v1.0.0-rc.1  # Dev build"; \
		echo "  make build CHART_MAJOR=v1 VERSION=v1.0.0       # Prod build"; \
		exit 1; \
	fi
	@# Detect build type from version tag (clean semver = prod, anything else = dev)
	@if echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		BUILD_TYPE=prod; \
	else \
		BUILD_TYPE=dev; \
	fi; \
	echo "Building $(CHART_MAJOR) version $(VERSION) ($$BUILD_TYPE)"; \
	echo ""; \
	\
	RANCHER_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".rancher-branch" config.yaml 2>/dev/null); \
	\
	CHART_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".upstream-refs.$$BUILD_TYPE.charts.branch" lock.yaml 2>/dev/null); \
	PARTNER_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".upstream-refs.$$BUILD_TYPE.partner.branch" lock.yaml 2>/dev/null); \
	RKE2_BRANCH=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".upstream-refs.$$BUILD_TYPE.rke2.branch" lock.yaml 2>/dev/null); \
	\
	CHART_COMMIT=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".upstream-refs.$$BUILD_TYPE.charts.commit" lock.yaml 2>/dev/null); \
	PARTNER_COMMIT=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".upstream-refs.$$BUILD_TYPE.partner.commit" lock.yaml 2>/dev/null); \
	RKE2_COMMIT=$$(yq eval ".chart-versions.\"$(CHART_MAJOR)\".upstream-refs.$$BUILD_TYPE.rke2.commit" lock.yaml 2>/dev/null); \
	\
	if [ "$$CHART_BRANCH" = "null" ] || [ -z "$$CHART_BRANCH" ]; then \
		echo "❌ Error: $$BUILD_TYPE refs not found in lock.yaml for $(CHART_MAJOR)"; \
		echo "Run 'make generate' first"; \
		exit 1; \
	fi; \
	\
	if [ "$$CHART_COMMIT" = "null" ] || [ -z "$$CHART_COMMIT" ]; then \
		echo "❌ Error: $$BUILD_TYPE commits not found in lock.yaml for $(CHART_MAJOR)"; \
		echo "Run 'make generate' first"; \
		exit 1; \
	fi; \
	\
	echo "Configuration:"; \
	echo "  Build type: $$BUILD_TYPE"; \
	echo "  Charts branch: $$CHART_BRANCH (commit: $${CHART_COMMIT:0:8})"; \
	echo "  Partner branch: $$PARTNER_BRANCH (commit: $${PARTNER_COMMIT:0:8})"; \
	echo "  RKE2 branch: $$RKE2_BRANCH (commit: $${RKE2_COMMIT:0:8})"; \
	echo "  Rancher branch: $$RANCHER_BRANCH"; \
	echo ""; \
	\
	if [ ! -f "$(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)" ]; then \
		echo "❌ Error: Dockerfile not found: $(DOCKERFILES_DIR)/Dockerfile.$(CHART_MAJOR)"; \
		echo "Run 'make generate' first"; \
		exit 1; \
	fi; \
	\
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
		--build-arg BUILD_URL="https://github.com/rancher/rancher-assets" \
		--tag "$(IMAGE_REPO):$(VERSION)" \
		--load \
		. && \
	echo "" && \
	echo "✅ Build complete: $(IMAGE_REPO):$(VERSION)"

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

build-release: ## Build release versions from lock.yaml (LOCAL DEBUG ONLY - use GHA for real releases)
	@echo "⚠️  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "⚠️  LOCAL DEBUG BUILD - NOT FOR PRODUCTION RELEASES"
	@echo "⚠️  Use GitHub Actions workflows for real releases"
	@echo "⚠️  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "Building release versions from lock.yaml..."
	@echo ""
	@CHART_MAJORS=$$(yq eval '.chart-versions | keys | .[]' lock.yaml); \
	if [ -z "$$CHART_MAJORS" ]; then \
		echo "❌ Error: No chart versions found in lock.yaml"; \
		exit 1; \
	fi; \
	BUILT_COUNT=0; \
	for major in $$CHART_MAJORS; do \
		VERSION=$$(yq eval ".chart-versions.\"$$major\".latest-stable" lock.yaml); \
		if [ "$$VERSION" = "null" ] || [ -z "$$VERSION" ]; then \
			echo "⏭️  Skipping $$major (no latest-stable set)"; \
			continue; \
		fi; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		echo "Building $$major at $$VERSION"; \
		echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
		$(MAKE) build CHART_MAJOR=$$major VERSION=$$VERSION; \
		if [ $$? -ne 0 ]; then \
			echo "❌ Build failed for $$major"; \
			exit 1; \
		fi; \
		BUILT_COUNT=$$((BUILT_COUNT + 1)); \
		echo ""; \
	done; \
	if [ $$BUILT_COUNT -eq 0 ]; then \
		echo "⚠️  No releases built - no chart versions have latest-stable set"; \
		exit 1; \
	fi; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	echo "✅ Built $$BUILT_COUNT release(s) - LOCAL DEBUG ONLY"; \
	echo "⚠️  Remember: Use GitHub Actions for production releases"; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

.PHONY: test
test: ## Run tests
	@go test -v ./...
