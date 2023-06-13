BUILD_DIR = build
DIST_DIR = dist
BIN_DIR = bin

OTELCOL_BUILDER_VERSION = 0.79.0
GORELEASER_VERSION = 1.18.2
OCB = go run go.opentelemetry.io/collector/cmd/builder@v$(OTELCOL_BUILDER_VERSION)
GORELEASER = go run github.com/goreleaser/goreleaser@v$(GORELEASER_VERSION)

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
test: $(BIN)
	go test ./...
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(BIN_DIR)
components: $(BIN)
	$(BIN) components


$(BIN):
	$(GORELEASER) build --single-target --snapshot --clean -o $(BIN)

$(BUILD_DIR)/main.go: 
	$(OCB) --config manifest.yaml --skip-compilation

$(EXE):  manifest.yaml
	$(OCB) --config manifest.yaml
