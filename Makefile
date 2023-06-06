BUILD_DIR = build
DEPS_DIR = deps

EXE = $(BUILD_DIR)/otelcol-dynatrace
OCB = $(DEPS_DIR)/ocb
OTELCOL_BUILDER_VERSION ?= 0.79.0

OS=$(shell uname | tr A-Z a-z)
MACHINE=$(shell uname -m)

.PHONY: build
build: $(EXE)

$(EXE): $(OCB) manifest.yaml
	$(OCB) --config manifest.yaml

$(DEPS_DIR):
	mkdir $(DEPS_DIR)

$(OCB): | $(DEPS_DIR)
	@{ \
	set -e ;\
	[ "$(MACHINE)" != x86 ] || machine=386 ;\
	[ "$(MACHINE)" != x86_64 ] || machine=amd64 ;\
	echo "Getting ocb ($(OS)/$(MACHINE))";\
	curl -sfLo $(OCB) "https://github.com/open-telemetry/opentelemetry-collector/releases/download/cmd%2Fbuilder%2Fv$(OTELCOL_BUILDER_VERSION)/ocb_$(OTELCOL_BUILDER_VERSION)_$(OS)_$${machine}" ;\
	chmod +x $(OCB) ;\
	}

.PHONY: test
test: $(EXE)
	go test ./...

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(DEPS_DIR)
