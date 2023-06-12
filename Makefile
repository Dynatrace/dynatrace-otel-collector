BUILD_DIR = build
DEPS_DIR = lib
DIST_DIR = dist
BIN_DIR = bin

OCB = $(DEPS_DIR)/ocb
OTELCOL_BUILDER_VERSION ?= 0.78.2

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

.PHONY: build test clean deps build components
build: $(BIN)
generate: $(BUILD_DIR)/main.go
deps: $(OCB)
test: $(BIN)
	go test ./...
clean:
	rm -rf $(BUILD_DIR) $(DEPS_DIR) $(DIST_DIR) $(BIN_DIR)
components: $(BIN)
	$(BIN) components


$(BIN): $(OCB)
	goreleaser build --single-target --snapshot --clean -o $(BIN)

$(BUILD_DIR)/main.go: $(OCB)
	$(OCB) --config manifest.yaml --skip-compilation

$(OCB):
	$(info OS=$(OS))
	$(info MACHINE=$(MACHINE))
	mkdir -p $(DEPS_DIR)
	curl -sfLo $(OCB) "https://github.com/open-telemetry/opentelemetry-collector/releases/download/cmd%2Fbuilder%2Fv$(OTELCOL_BUILDER_VERSION)/ocb_$(OTELCOL_BUILDER_VERSION)_$(OS)_$(MACHINE)"
	chmod +x $(OCB)

$(EXE): $(OCB) manifest.yaml
	$(OCB) --config manifest.yaml

$(DEPS_DIR):
	mkdir $(DEPS_DIR)
