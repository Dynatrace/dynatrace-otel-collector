# Tool management logic from:
# https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/820510e537167f621c857caaa0109f0dad021d74/Makefile.Common

BUILD_DIR = build
DIST_DIR = dist
BIN_DIR = bin

# SRC_ROOT is the top of the source tree.
SRC_ROOT := $(shell git rev-parse --show-toplevel)

ifeq ($(OS),Windows_NT)
	OS = windows
    ifeq ($(PROCESSOR_ARCHITECTURE),AMD64)
        MACHINE = amd64
    endif
    ifeq ($(PROCESSOR_ARCHITECTURE),x86)
        MACHINE = 386
    endif
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        OS = linux
    endif
    ifeq ($(UNAME_S),Darwin)
        OS = darwin
    endif
    UNAME_M := $(shell uname -m)
    ifeq ($(UNAME_M),x86)
        MACHINE = 386
    endif
    ifeq ($(UNAME_M),x86_64)
        MACHINE = amd64
    endif
    ifneq ($(filter arm%,$(UNAME_M)),)
        MACHINE = arm64
    endif
endif

BIN = $(BIN_DIR)/oteltestbedcol_$(OS)_$(MACHINE)
MAIN = $(BUILD_DIR)/main.go

# Files to be copied directly from the project root
CP_FILES = LICENSE README.md
CP_FILES_DEST = $(addprefix $(BUILD_DIR)/, $(CP_FILES))

TOOLS_MOD_DIR    := $(SRC_ROOT)/internal/tools
TOOLS_MOD_REGEX  := "\s+_\s+\".*\""
TOOLS_PKG_NAMES  := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"")
TOOLS_BIN_DIR    := $(SRC_ROOT)/.tools
TOOLS_BIN_NAMES  := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(TOOLS_PKG_NAMES)))

GORELEASER := $(TOOLS_BIN_DIR)/goreleaser
BUILDER    := $(TOOLS_BIN_DIR)/builder

.PHONY: build test clean components install-tools
build: $(BIN) $(CP_FILES_DEST)
generate: $(MAIN) $(CP_FILES_DEST)
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

$(MAIN): $(BUILDER)
	$(BUILDER) --config manifest.yaml --skip-compilation

$(EXE): manifest.yaml $(BUILDER) 
	$(BUILDER) --config manifest.yaml

$(CP_FILES_DEST): $(MAIN)
	cp $(notdir $@) $@
