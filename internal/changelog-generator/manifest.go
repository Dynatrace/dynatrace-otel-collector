package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ChloggenContribURL = "https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector-contrib/main/.chloggen/config.yaml"
	ChloggenCoreURL    = "https://raw.githubusercontent.com/open-telemetry/opentelemetry-collector/main/.chloggen/config.yaml"
)

// Manifest mirrors the structure of manifest.yaml relevant for component extraction.
type Manifest struct {
	Dist struct {
		Version string `yaml:"version"`
	} `yaml:"dist"`
	Receivers  []GoMod `yaml:"receivers"`
	Exporters  []GoMod `yaml:"exporters"`
	Extensions []GoMod `yaml:"extensions"`
	Processors []GoMod `yaml:"processors"`
	Connectors []GoMod `yaml:"connectors"`
	Providers  []GoMod `yaml:"providers"`
}

// GoMod wraps a single gomod entry.
type GoMod struct {
	GoMod string `yaml:"gomod"`
}

// ChloggenConfig holds the components list from upstream .chloggen/config.yaml.
type ChloggenConfig struct {
	Components []string `yaml:"components"`
}

// ParseChloggenConfig fetches and parses one or more chloggen configs from URLs
// or local paths, returning a map of normalized ID -> canonical chloggen ID.
// e.g. "receiver/filelog" -> "receiver/file_log"
func ParseChloggenConfig(urlsOrPaths ...string) (map[string]string, error) {
	index := make(map[string]string)
	for _, urlOrPath := range urlsOrPaths {
		var data []byte
		var err error

		if strings.HasPrefix(urlOrPath, "http") {
			resp, err := http.Get(urlOrPath)
			if err != nil {
				return nil, fmt.Errorf("fetching chloggen config %s: %w", urlOrPath, err)
			}
			defer resp.Body.Close()
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("reading chloggen response %s: %w", urlOrPath, err)
			}
		} else {
			data, err = os.ReadFile(urlOrPath)
			if err != nil {
				return nil, fmt.Errorf("reading chloggen config %s: %w", urlOrPath, err)
			}
		}

		var cfg ChloggenConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing chloggen config %s: %w", urlOrPath, err)
		}

		for _, c := range cfg.Components {
			normalized := strings.ReplaceAll(c, "_", "")
			index[normalized] = c
		}
	}
	return index, nil
}

// ParseManifest reads manifest.yaml and returns the set of canonical upstream
// chloggen component IDs present in the manifest, plus the dist version.
// Only components found in the provided chloggen index are included.
func ParseManifest(path string, index map[string]string) (components map[string]bool, distVersion string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, "", fmt.Errorf("parsing manifest: %w", err)
	}

	components = make(map[string]bool)
	addComponents(components, "receiver", m.Receivers, index)
	addComponents(components, "exporter", m.Exporters, index)
	addComponents(components, "extension", m.Extensions, index)
	addComponents(components, "processor", m.Processors, index)
	addComponents(components, "connector", m.Connectors, index)
	// Providers are not referenced in upstream changelogs — skip.

	return components, m.Dist.Version, nil
}

func addComponents(dst map[string]bool, compType string, mods []GoMod, index map[string]string) {
	for _, m := range mods {
		derived := gomodToComponentID(m.GoMod, compType)
		if derived == "" {
			continue
		}
		normalized := strings.ReplaceAll(derived, "_", "")
		if canonical, ok := index[normalized]; ok {
			dst[canonical] = true
		}
		// Not found in chloggen — not an upstream component, skip.
	}
}

// gomodToComponentID converts a gomod string + component type into a
// derived component ID (e.g. "receiver/filelog").
// This is an intermediate form — the canonical chloggen ID is resolved
// via the chloggen index.
//
// Input example:
//
//	gomod:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.145.0"
//	compType: "receiver"
//	result:   "receiver/filelog"
func gomodToComponentID(gomod, compType string) string {
	parts := strings.Fields(gomod)
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]

	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return ""
	}
	rawName := segments[len(segments)-1]

	strippedName := stripTypeSuffix(rawName, compType)
	return compType + "/" + strippedName
}

// stripTypeSuffix removes the component-type suffix from a raw package name.
// If the name does not end with the suffix it is returned unchanged.
func stripTypeSuffix(name, compType string) string {
	if strings.HasSuffix(name, compType) {
		return name[:len(name)-len(compType)]
	}
	return name
}
