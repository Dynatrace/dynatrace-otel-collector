// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ConfigTemplate struct {
	Host      string
	Namespace string
	// Overlays are YAML strings applied in order; later ones win.
	Templates []string
}

func GetCollectorConfig(path string, template ConfigTemplate) (string, error) {
	if path == "" {
		return "", nil
	}

	baseBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	replacer := strings.NewReplacer(
		"${env:DT_ENDPOINT}", fmt.Sprintf("http://%s:4318", template.Host),
		"${env:DT_API_TOKEN}", "",
		"${env:API_TOKEN}", "",
		"${env:NAMESPACE}", template.Namespace,
	)
	baseStr := replacer.Replace(string(baseBytes))

	for i, ov := range template.Templates {
		if strings.TrimSpace(ov) == "" {
			continue
		}
		ov = replacer.Replace(ov)
		baseStr, err = mergeYAMLText(baseStr, ov)
		if err != nil {
			return "", fmt.Errorf("applying overlay %d: %w", i, err)
		}
	}

	// Indent for ConfigMap
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(baseStr))
	for sc.Scan() {
		b.WriteString("    ")
		b.WriteString(sc.Text())
		b.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("building indented config: %w", err)
	}
	return b.String(), nil
}

// mergeYAMLText merges overlay into base at the text level.
// For each top-level key in overlay, it replaces the entire block
// under that key in base with the overlay's version — no re-marshaling.
func mergeYAMLText(base, overlay string) (string, error) {
	// Parse overlay to find which top-level keys it touches
	var overlayMap yaml.Node
	if err := yaml.Unmarshal([]byte(overlay), &overlayMap); err != nil {
		return "", fmt.Errorf("parse overlay: %w", err)
	}
	if len(overlayMap.Content) == 0 {
		return base, nil
	}

	// Top-level keys in overlay
	overlayKeys := map[string]struct{}{}
	root := overlayMap.Content[0]
	for i := 0; i < len(root.Content)-1; i += 2 {
		overlayKeys[root.Content[i].Value] = struct{}{}
	}

	// Split base into top-level sections, replacing touched ones
	result := replaceTopLevelSections(base, overlay, overlayKeys)
	return result, nil
}

// replaceTopLevelSections replaces sections in base whose top-level key
// appears in overlayKeys with the corresponding block from overlay.
// Keys in base not present in overlay are kept as-is.
// Keys in overlay not present in base are appended.
func replaceTopLevelSections(base, overlay string, overlayKeys map[string]struct{}) string {
	baseSections := splitTopLevelSections(base)
	overlaySections := splitTopLevelSections(overlay)

	seen := map[string]bool{}
	var out strings.Builder

	for _, sec := range baseSections {
		if _, replace := overlayKeys[sec.key]; replace {
			// Use overlay's version of this section
			if ovSec, ok := overlaySections[sec.key]; ok {
				out.WriteString(ovSec.raw)
				out.WriteString("\n")
			}
			seen[sec.key] = true
		} else {
			out.WriteString(sec.raw)
			out.WriteString("\n")
		}
	}

	// Append overlay keys not present in base
	for key, ovSec := range overlaySections {
		if !seen[key] {
			out.WriteString(ovSec.raw)
			out.WriteString("\n")
		}
	}

	return strings.TrimRight(out.String(), "\n") + "\n"
}

type section struct {
	key string
	raw string
}

// splitTopLevelSections splits a YAML string into top-level key blocks.
// Each block starts at a line matching /^[a-z_]+:/ and ends before the next such line.
func splitTopLevelSections(text string) map[string]section {
	lines := strings.Split(text, "\n")
	sections := map[string]section{}
	var currentKey string
	var currentLines []string

	flush := func() {
		if currentKey != "" {
			raw := strings.TrimRight(strings.Join(currentLines, "\n"), "\n")
			sections[currentKey] = section{key: currentKey, raw: raw}
		}
	}

	for _, line := range lines {
		// Top-level key: starts with a letter/underscore, no leading spaces, ends with ':'
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' && line[0] != '-' && strings.Contains(line, ":") {
			flush()
			currentKey = strings.TrimRight(strings.SplitN(line, ":", 2)[0], " ")
			currentLines = []string{line}
		} else if currentKey != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()
	return sections
}

func KubeconfigFromEnvOrDefault() string {
	if fromEnv := os.Getenv(KubeConfigEnvVar); fromEnv != "" {
		return fromEnv
	}
	return TestKubeConfig
}

func CreateObjectFromFile(t *testing.T, client *xk8stest.K8sClient, file string) *unstructured.Unstructured {
	buf, err := os.ReadFile(file)
	require.NoErrorf(t, err, "failed to read object file %s", file)

	obj, err := xk8stest.CreateObject(client, buf)
	require.NoErrorf(t, err, "failed to create k8s object from file %s", file)

	t.Cleanup(func() {
		require.NoErrorf(t, xk8stest.DeleteObject(client, obj), "failed to delete object %s", obj.GetName())
	})
	return obj
}

func CreateCollectorObjects(t *testing.T, client *xk8stest.K8sClient, testID string, manifestsDir string, values map[string]string, host string) []*unstructured.Unstructured {
	objs := xk8stest.CreateCollectorObjects(t, client, testID, manifestsDir, values, host)
	t.Cleanup(func() {
		for _, obj := range objs {
			require.NoErrorf(t, xk8stest.DeleteObject(client, obj), "failed to delete object %s", obj.GetName())
		}
	})
	return objs
}

// Helper to read overlay file content as string
func MustRead(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	require.NoErrorf(t, err, "read file %s", p)
	return string(b)
}
