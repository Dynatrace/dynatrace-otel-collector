# Tool management logic from:
# https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/820510e537167f621c857caaa0109f0dad021d74/Makefile.Common

include ./Makefile.Common

BUILD_DIR = build
DIST_DIR = dist
BIN_DIR = bin

# ALL_MODULES includes ./* dirs (excludes . dir)
ALL_MODS := $(shell find . -type f -name "go.mod" -not -path "./build/*" -not -path "./internal/tools/*" -not -path "./internal/testing-setups/*" -exec dirname {} \; | sort | grep -E '^./' )
# INTERNAL_MODS includes only ./internal/* dirs
INTERNAL_MODS := $(shell find ./internal/* -type f -name "go.mod" -exec dirname {} \; | sort | grep -E '^./' )

BIN = $(BIN_DIR)/dynatrace-otel-collector
MAIN = $(BUILD_DIR)/main.go

# Files to be copied directly from the project root
CP_FILES = LICENSE README.md
CP_FILES_DEST = $(addprefix $(BUILD_DIR)/, $(CP_FILES))

PACKAGE_PATH ?= ""
ARCH ?= ""

CHLOGGEN_CONFIG := .chloggen/config.yaml

# renovate: datasource=github-releases depName=open-telemetry/opentelemetry-collector-contrib
OTEL_UPSTREAM_VERSION=v0.157.0

.PHONY: build generate test package-test components snapshot
build: $(BIN)
build-all: .goreleaser.yaml $(GORELEASER) $(MAIN)
	$(GORELEASER) build --snapshot --clean
generate: $(MAIN) $(CP_FILES_DEST)
test: $(BIN)
	@result=0; \
	for MOD in $(ALL_MODS); do \
		cd $${MOD}; \
		go test -v ./... || result=1; \
		cd -; \
	done; \
	exit $$result;
package-test:
	./internal/testbed/linux-services/package-tests.sh $(PACKAGE_PATH) $(ARCH)
components: $(BIN)
	$(BIN) components

snapshot: .goreleaser.yaml $(GORELEASER)
	$(GORELEASER) release --snapshot --clean --skip archive,sbom --fail-fast

$(TOOLS_BIN_NAMES): $(TOOLS_MOD_DIR)/go.mod | $(TOOLS_BIN_DIR)
	cd $(TOOLS_MOD_DIR) && $(GOCMD) build -trimpath -o $(abspath $@) \
		$(filter %/$(notdir $@),$(TOOLS_PKG_NAMES))

$(BIN): .goreleaser.yaml $(GORELEASER) $(MAIN)
	$(GORELEASER) build --single-target --snapshot --clean -o $(BIN)

$(MAIN): $(BUILDER) manifest.yaml
	$(BUILDER) --config manifest.yaml --skip-compilation

$(CP_FILES_DEST): $(MAIN)
	cp $(notdir $@) $@

.PHONY: gotidy
gotidy:
	$(MAKE) --no-print-directory for-all-target TARGET="modtidy"

ifeq ($(GOOS),windows)
	EXTENSION := .exe
endif

.PHONY: oteltestbedcol
oteltestbedcol: genoteltestbedcol
	cd ./cmd/oteltestbedcol && GO111MODULE=on CGO_ENABLED=0 go build -trimpath -o ../../bin/oteltestbedcol_$(GOOS)_$(GOARCH)$(EXTENSION) .

# 1. Copy and modify the manifest -> move the modified file to the cmd/oteltestbedcol directory
# 2. Add pprofextension used for load tests to the test manifest in cmd/oteltestbedcol directory
# 3. Generate code
.PHONY: genoteltestbedcol
genoteltestbedcol: $(BUILDER)
	cp manifest.yaml cmd/oteltestbedcol/manifest.yaml
	awk '/healthcheckextension $(OTEL_UPSTREAM_VERSION)/ {print; print "  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension $(OTEL_UPSTREAM_VERSION)"; next}1' cmd/oteltestbedcol/manifest.yaml > cmd/oteltestbedcol/manifest-dev.yaml
	$(BUILDER) --skip-compilation --config cmd/oteltestbedcol/manifest-dev.yaml --output-path cmd/oteltestbedcol

.PHONY: run-load-tests
run-load-tests:
	mkdir -p ./internal/testbed/bin/
	cp -a ./bin/oteltestbedcol_$(GOOS)_$(GOARCH)$(EXTENSION) ./internal/testbed/bin/
	$(MAKE) --no-print-directory -C internal/testbed/load run-tests

# Define a delegation target for each module
.PHONY: $(INTERNAL_MODS)
$(INTERNAL_MODS):
	@echo "Running target '$(TARGET)' in module '$@' as part of group '$(GROUP)'"
	$(MAKE) --no-print-directory -C $@ $(TARGET)

# Trigger each module's delegation target
.PHONY: for-all-target
for-all-target: $(INTERNAL_MODS)

.PHONY: gomoddownload
gomoddownload:
	$(MAKE) --no-print-directory for-all-target TARGET="moddownload"

E2E_IMAGE_TAG      ?= e2e-test
E2E_IMAGE          := dynatrace-otel-collector:$(E2E_IMAGE_TAG)
E2E_IMAGE_JRN      := dynatrace-otel-collector-journald:$(E2E_IMAGE_TAG)
E2E_KUBECONFIG     := /tmp/kube-config-collector-e2e-testing
E2E_K8S_VERSION    := v1.36.1
E2E_CLUSTER_NAME   := kind
E2E_CLUSTER_CONFIG ?= .github/actions/create-cluster/single-node.yaml
SKIP_E2E_BUILD     ?= false
E2E_DOCKER_CTX     := /tmp/dynatrace-otel-collector-e2e-build
E2E_LINUX_ARCH     := $(shell docker info --format '{{.Architecture}}' 2>/dev/null | sed 's/aarch64/arm64/;s/x86_64/amd64/')

E2E_SUITES_SINGLE := k8senrichment prometheus zipkin statsd redaction bearertokenauth \
                     resource-detection netflow self-monitoring self-monitoring-prometheus \
                     k8sobjects kubeletstats k8scluster loadbalancing kafka filestorage \
                     genainormalizer hostmetrics

E2E_SUITES_MULTI := k8scombined prometheus-large-scale

.PHONY: e2e-build e2e-test e2e
e2e-build: generate
	mkdir -p $(E2E_DOCKER_CTX)
	cd build && GOOS=linux GOARCH=$(E2E_LINUX_ARCH) go build -trimpath \
		-o $(E2E_DOCKER_CTX)/dynatrace-otel-collector .
	docker build -t $(E2E_IMAGE) -f Dockerfile $(E2E_DOCKER_CTX)/
	docker build -t $(E2E_IMAGE_JRN) -f Dockerfile-journald $(E2E_DOCKER_CTX)/

e2e-test:
	@[ "$(SKIP_E2E_BUILD)" = "true" ] || $(MAKE) e2e-build
	@[ -n "$(SUITE)" ] || { echo "Error: SUITE is required. Usage: make e2e-test SUITE=<suite>"; exit 1; }
	@trap 'kind delete cluster --name $(E2E_CLUSTER_NAME) 2>/dev/null; true' EXIT; \
	kind delete cluster --name $(E2E_CLUSTER_NAME) 2>/dev/null || true; \
	kind create cluster \
		--name $(E2E_CLUSTER_NAME) \
		--image kindest/node:$(E2E_K8S_VERSION) \
		--kubeconfig $(E2E_KUBECONFIG) \
		--config $(E2E_CLUSTER_CONFIG); \
	kind load docker-image $(E2E_IMAGE) --name $(E2E_CLUSTER_NAME); \
	kind load docker-image $(E2E_IMAGE_JRN) --name $(E2E_CLUSTER_NAME); \
	docker exec $(E2E_CLUSTER_NAME)-control-plane bash -c \
		'dd if=/dev/zero of=/var/swapfile bs=1M count=512 status=none && chmod 600 /var/swapfile && mkswap /var/swapfile && swapon /var/swapfile' 2>/dev/null || true; \
	cd internal/testbed/integration/$(SUITE) && \
	KUBECONFIG=$(E2E_KUBECONFIG) go test -v --tags=e2e

e2e: e2e-build
	@for suite in $(E2E_SUITES_SINGLE); do \
		$(MAKE) --no-print-directory e2e-test SUITE=$$suite SKIP_E2E_BUILD=true; \
	done
	@for suite in $(E2E_SUITES_MULTI); do \
		$(MAKE) --no-print-directory e2e-test SUITE=$$suite SKIP_E2E_BUILD=true \
			E2E_CLUSTER_CONFIG=.github/actions/create-cluster/multi-node.yaml; \
	done

OUT_BASE ?= /tmp/rendered-collectors-workloads

RENDERWORKLOADS_MOD_DIR := internal/renderworkloads

.PHONY: render-workloads kyverno-workloads

render-workloads:
	@echo "Rendering workloads to $(OUT_BASE)"
	@cd "$(SRC_ROOT)/$(RENDERWORKLOADS_MOD_DIR)" && go run . \
		-repo-root "$(SRC_ROOT)" \
		-in-root internal/testbed/integration \
		-out-base "$(OUT_BASE)" \
		-vars-file internal/renderworkloads/render-vars.json \


kyverno-workloads: render-workloads
	@echo "Running Kyverno against rendered workloads from $(OUT_BASE)/workloads.txt"
	@cd "$(SRC_ROOT)" && \
		{ test -s "$(OUT_BASE)/workloads.txt" || { echo "ERROR: workloads.txt is empty"; exit 1; }; } && \
		sed 's|^|-r |' "$(OUT_BASE)/workloads.txt" \
		| xargs -n 1000 kyverno apply .github/workflows/kyverno/policies/*.yaml
