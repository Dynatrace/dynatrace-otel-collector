# Tool management logic from:
# https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/820510e537167f621c857caaa0109f0dad021d74/Makefile.Common

BUILD_DIR = build
DIST_DIR = dist
BIN_DIR = bin

# SRC_ROOT is the top of the source tree.
SRC_ROOT := $(shell git rev-parse --show-toplevel)

BIN = $(BIN_DIR)/otelcol-dynatrace

TOOLS_MOD_DIR    := $(SRC_ROOT)/internal/tools
TOOLS_MOD_REGEX  := "\s+_\s+\".*\""
TOOLS_PKG_NAMES  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"")
TOOLS_BIN_DIR    := $(SRC_ROOT)/.tools
TOOLS_BIN_NAMES  := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(TOOLS_PKG_NAMES)))

GORELEASER := $(TOOLS_BIN_DIR)/goreleaser
BUILDER    := $(TOOLS_BIN_DIR)/builder

.PHONY: build test clean components install-tools
build: $(BIN)
generate: $(BUILD_DIR)/main.go
test: $(BIN)
	go test ./...
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(BIN_DIR) $(TOOLS_BIN_DIR)
components: $(BIN)
	$(BIN) components
install-tools: $(TOOLS_BIN_NAMES)

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_BIN_NAMES): $(TOOLS_MOD_DIR)/go.mod $(TOOLS_BIN_DIR)
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES))

$(BIN): .goreleaser.yaml $(GORELEASER)
	$(GORELEASER) build --single-target --snapshot --clean -o $(BIN)

$(BUILD_DIR)/main.go: $(BUILDER)
	$(BUILDER) --config manifest.yaml --skip-compilation

$(EXE): manifest.yaml $(BUILDER) 
	$(BUILDER) --config manifest.yaml

