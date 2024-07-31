# Tool management logic from:
# https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/820510e537167f621c857caaa0109f0dad021d74/Makefile.Common

BUILD_DIR = build
DIST_DIR = dist
BIN_DIR = bin

# SRC_ROOT is the top of the source tree.
SRC_ROOT := $(shell git rev-parse --show-toplevel)

# ALL_MODULES includes ./* dirs (excludes . dir)
ALL_MODULES := $(shell find . -type f -name "go.mod" -not -path "./build/*" -not -path "./internal/tools/*" -exec dirname {} \; | sort | grep -E '^./' )
# Append root module to all modules
GOMODULES = $(ALL_MODULES)

SOURCES := $(shell find internal/confmap -type f | sort )

BIN = $(BIN_DIR)/dynatrace-otel-collector
MAIN = $(BUILD_DIR)/main.go

# Files to be copied directly from the project root
CP_FILES = LICENSE README.md
CP_FILES_DEST = $(addprefix $(BUILD_DIR)/, $(CP_FILES))

TOOLS_MOD_DIR    := $(SRC_ROOT)/internal/tools
TOOLS_MOD_REGEX  := "\s+_\s+\".*\""
TOOLS_PKG_NAMES  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"")
TOOLS_BIN_DIR    := $(SRC_ROOT)/.tools
TOOLS_BIN_NAMES  := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(TOOLS_PKG_NAMES)))

GORELEASER := $(TOOLS_BIN_DIR)/v2
BUILDER    := $(TOOLS_BIN_DIR)/builder
CHLOGGEN   := $(TOOLS_BIN_DIR)/chloggen

CHLOGGEN_CONFIG := .chloggen/config.yaml

.PHONY: build generate test package-test clean clean-all components install-tools snapshot release
build: $(BIN)
build-all: .goreleaser.yaml $(GORELEASER) $(MAIN)
	$(GORELEASER) build --snapshot --clean
generate: $(MAIN) $(CP_FILES_DEST)
test: $(BIN)
	@result=0; \
	for MOD in $(GOMODULES); do \
		cd $${MOD}; \
		go test -v ./... || result=1; \
		cd -; \
	done; \
	exit $$result;
package-test:
	./internal/testbed/linux-services/package-tests.sh dist/dynatrace-otel-collector_0.10.1-next_Linux_arm64.deb
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(BIN_DIR)
clean-tools:
	rm -rf $(TOOLS_BIN_DIR)
clean-all: clean clean-tools
components: $(BIN)
	$(BIN) components
install-tools: $(TOOLS_BIN_NAMES)
snapshot: .goreleaser.yaml $(GORELEASER)
	$(GORELEASER) release --snapshot --clean --skip sbom,sign,archive,docker
release: .goreleaser.yaml $(GORELEASER)
	$(GORELEASER) release --clean

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_BIN_NAMES): $(TOOLS_MOD_DIR)/go.mod | $(TOOLS_BIN_DIR)
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES))

$(BIN): .goreleaser.yaml $(GORELEASER) $(MAIN) $(SOURCES)
	$(GORELEASER) build --single-target --snapshot --clean -o $(BIN)

$(MAIN): $(BUILDER) manifest.yaml
	$(BUILDER) --config manifest.yaml --skip-compilation

$(CP_FILES_DEST): $(MAIN)
	cp $(notdir $@) $@

FILENAME?=$(shell git branch --show-current)
.PHONY: chlog-new
chlog-new: $(CHLOGGEN)
	$(CHLOGGEN) new --config $(CHLOGGEN_CONFIG) --filename $(FILENAME)

.PHONY: chlog-validate
chlog-validate: $(CHLOGGEN)
	$(CHLOGGEN) validate --config $(CHLOGGEN_CONFIG)

.PHONY: chlog-preview
chlog-preview: $(CHLOGGEN)
	$(CHLOGGEN) update --config $(CHLOGGEN_CONFIG) --dry

.PHONY: chlog-update
chlog-update: $(CHLOGGEN)
	$(CHLOGGEN) update --config $(CHLOGGEN_CONFIG) --version $(VERSION)
