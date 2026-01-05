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

SOURCES := $(shell find internal/confmap -type f | sort )

BIN = $(BIN_DIR)/dynatrace-otel-collector
MAIN = $(BUILD_DIR)/main.go

# Files to be copied directly from the project root
CP_FILES = LICENSE README.md
CP_FILES_DEST = $(addprefix $(BUILD_DIR)/, $(CP_FILES))

PACKAGE_PATH ?= ""
ARCH ?= ""

CHLOGGEN_CONFIG := .chloggen/config.yaml

# renovate: datasource=github-releases depName=open-telemetry/opentelemetry-collector-contrib
OTEL_UPSTREAM_VERSION=v0.142.0

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
	$(GORELEASER) release --snapshot --clean --parallelism 2 --skip archive,sbom --fail-fast

$(TOOLS_BIN_NAMES): $(TOOLS_MOD_DIR)/go.mod | $(TOOLS_BIN_DIR)
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES))

$(BIN): .goreleaser.yaml $(GORELEASER) $(MAIN) $(SOURCES)
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

# 1. Copy and modify the manifest -> change local path to eecprovider -> move the modified file to the cmd/oteltestbedcol directory
# 2. Add pprofextension used for load tests to the test manifest in cmd/oteltestbedcol directory
# 3. Generate code
.PHONY: genoteltestbedcol
genoteltestbedcol: $(BUILDER)
	awk '{gsub(/\.\.\/internal\/confmap\/provider\/eecprovider/, "../../internal/confmap/provider/eecprovider"); print}' manifest.yaml > cmd/oteltestbedcol/manifest.yaml
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
