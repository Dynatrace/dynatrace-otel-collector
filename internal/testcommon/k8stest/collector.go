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

// mergeNodes merges src yaml.Node into dst yaml.Node, both must be mapping nodes.
// Keys present in src are added or overwrite dst; missing keys are left as-is.
// This preserves the original formatting/style of dst for untouched keys.
func mergeNodes(dst, src *yaml.Node) {
	// Build an index of dst mapping keys → value-node positions
	index := map[string]int{}
	for i := 0; i < len(dst.Content)-1; i += 2 {
		index[dst.Content[i].Value] = i
	}

	for i := 0; i < len(src.Content)-1; i += 2 {
		key := src.Content[i]
		val := src.Content[i+1]

		if pos, exists := index[key.Value]; exists {
			// Key exists in dst
			dstVal := dst.Content[pos+1]
			if val.Kind == yaml.MappingNode && dstVal.Kind == yaml.MappingNode {
				// Both are maps: recurse
				mergeNodes(dstVal, val)
			} else {
				// Scalar or sequence: src wins, replace value node only
				dst.Content[pos+1] = val
			}
		} else {
			// New key: append both key and value nodes from src
			dst.Content = append(dst.Content, key, val)
		}
	}
}

func GetCollectorConfig(path string, template ConfigTemplate) (string, error) {
	if path == "" {
		return "", nil
	}

	baseBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// 1) Parse base YAML preserving structure via yaml.Node
	var baseDoc yaml.Node
	if err := yaml.Unmarshal(baseBytes, &baseDoc); err != nil {
		return "", fmt.Errorf("unmarshal base config %q: %w", path, err)
	}
	// baseDoc is a Document node; its first child is the root mapping
	root := baseDoc.Content[0]

	// 2) Apply overlays in order using node-level merge
	for i, ov := range template.Templates {
		if strings.TrimSpace(ov) == "" {
			continue
		}
		var overlayDoc yaml.Node
		if err := yaml.Unmarshal([]byte(ov), &overlayDoc); err != nil {
			return "", fmt.Errorf("unmarshal overlay %d: %w", i, err)
		}
		mergeNodes(root, overlayDoc.Content[0])
	}

	// 3) Marshal merged node back — preserves key names (including slashes) exactly
	mergedBytes, err := yaml.Marshal(&baseDoc)
	if err != nil {
		return "", fmt.Errorf("marshal merged config: %w", err)
	}
	merged := string(mergedBytes)

	// 4) Apply env/host/namespace replacements
	replacer := strings.NewReplacer(
		"${env:DT_ENDPOINT}", fmt.Sprintf("http://%s:4318", template.Host),
		"${env:DT_API_TOKEN}", "",
		"${env:API_TOKEN}", "",
		"${env:NAMESPACE}", template.Namespace,
	)
	parsedConfig := replacer.Replace(merged)

	// 5) Indent for ConfigMap
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(parsedConfig))
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
