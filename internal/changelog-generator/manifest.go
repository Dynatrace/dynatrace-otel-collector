package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
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

// ParseManifest reads manifest.yaml and returns a set of upstream component IDs
// (e.g. "receiver/filelog", "processor/batch") and the dist version.
func ParseManifest(path string) (components map[string]bool, distVersion string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, "", fmt.Errorf("parsing manifest: %w", err)
	}

	components = make(map[string]bool)
	addComponents(components, "receiver", m.Receivers)
	addComponents(components, "exporter", m.Exporters)
	addComponents(components, "extension", m.Extensions)
	addComponents(components, "processor", m.Processors)
	addComponents(components, "connector", m.Connectors)
	// Providers are not referenced in upstream changelogs — skip.

	return components, m.Dist.Version, nil
}

func addComponents(dst map[string]bool, compType string, mods []GoMod) {
	for _, m := range mods {
		id := gomodToComponentID(m.GoMod, compType)
		if id != "" {
			dst[id] = true
		}
	}
}

// gomodToComponentID converts a gomod string + component type into the
// upstream chloggen component ID format (e.g. "receiver/filelog").
//
// Input example:
//
//	gomod:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.145.0"
//	compType: "receiver"
//	result:   "receiver/filelog"
func gomodToComponentID(gomod, compType string) string {
	// Split off version suffix.
	parts := strings.Fields(gomod)
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]

	// Extract the last path segment.
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return ""
	}
	rawName := segments[len(segments)-1]

	// Strip the type suffix from the raw name (e.g. "filelogreceiver" → "filelog").
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
