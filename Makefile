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

# renovate: datasource=github-releases depName=jstemmer/go-junit-report
GO_JUNIT_REPORT_VERSION?=v2.1.0

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

PACKAGE_PATH ?= ""
ARCH ?= ""

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
	./internal/testbed/linux-services/package-tests.sh $(PACKAGE_PATH) $(ARCH)
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(BIN_DIR)
clean-tools:
	rm -rf $(TOOLS_BIN_DIR)
clean-all: clean clean-tools
components: $(BIN)
	$(BIN) components
install-tools: install-go-junit-report $(TOOLS_BIN_NAMES)
snapshot: .goreleaser.yaml $(GORELEASER)
	$(GORELEASER) release --snapshot --clean
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

.PHONY: gotidy
gotidy:
	$(MAKE) --no-print-directory for-all-target TARGET="modtidy"

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

SRC_ROOT := $(shell git rev-parse --show-toplevel)

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
ifeq ($(GOOS),windows)
	EXTENSION := .exe
endif

.PHONY: oteltestbedcol
oteltestbedcol:
	cd ./internal/oteltestbedcol && GO111MODULE=on CGO_ENABLED=0 go build -trimpath -o ../../bin/oteltestbedcol_$(GOOS)_$(GOARCH)$(EXTENSION) .

GOJUNIT = .tools/go-junit-report

.PHONY: run-load-tests
run-load-tests:
	mkdir -p ./internal/testbed/bin/
	cp -a ./bin/oteltestbedcol_$(GOOS)_$(GOARCH)$(EXTENSION) ./internal/testbed/bin/
	PWD=$(pwd)
	GOJUNIT="$(PWD)/$(GOJUNIT)" $(MAKE) --no-print-directory -C internal/testbed/load run-tests

FIND_MOD_ARGS=-type f -name "go.mod"
TO_MOD_DIR=dirname {} \; | sort | grep -E '^./'
INTERNAL_MODS := $(shell find ./internal/* $(FIND_MOD_ARGS) -exec $(TO_MOD_DIR) )
ALL_MODS := $(INTERNAL_MODS)

# Define a delegation target for each module
.PHONY: $(ALL_MODS)
$(ALL_MODS):
	@echo "Running target '$(TARGET)' in module '$@' as part of group '$(GROUP)'"
	$(MAKE) --no-print-directory -C $@ $(TARGET)

# Trigger each module's delegation target
.PHONY: for-all-target
for-all-target: $(ALL_MODS)

.PHONY: gomoddownload
gomoddownload:
	$(MAKE) --no-print-directory for-all-target TARGET="moddownload"

.PHONY: install-go-junit-report
install-go-junit-report:
	GOBIN=$(TOOLS_BIN_DIR) go install github.com/jstemmer/go-junit-report/v2@$(GO_JUNIT_REPORT_VERSION)
