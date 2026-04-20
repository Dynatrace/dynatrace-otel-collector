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

// mergeMaps merges src into dst. Both must be map[string]any.
// - map keys are merged recursively
// - lists and scalars are replaced wholesale (src wins)
func mergeMaps(dst, src map[string]any) map[string]any {
	for k, v := range src {
		// If both sides are maps, merge recursively
		if vMap, ok := v.(map[string]any); ok {
			if dv, ok := dst[k]; ok {
				if dvMap, ok := dv.(map[string]any); ok {
					dst[k] = mergeMaps(dvMap, vMap)
					continue
				}
			}
		}
		// Otherwise, src wins
		dst[k] = v
	}
	return dst
}
func GetCollectorConfig(path string, template ConfigTemplate) (string, error) {
	if path == "" {
		return "", nil
	}

	baseBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// 1) Unmarshal base YAML into a map
	var cfg map[string]any
	if err := yaml.Unmarshal(baseBytes, &cfg); err != nil {
		return "", fmt.Errorf("unmarshal base config %q: %w", path, err)
	}

	// 2) Apply overlays in order: later overlays win
	for i, ov := range template.Templates {
		if strings.TrimSpace(ov) == "" {
			continue
		}

		var overlayCfg map[string]any
		if err := yaml.Unmarshal([]byte(ov), &overlayCfg); err != nil {
			return "", fmt.Errorf("unmarshal overlay %d: %w", i, err)
		}

		cfg = mergeMaps(cfg, overlayCfg)
	}

	// 3) Marshal merged YAML back to bytes
	mergedBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal merged config: %w", err)
	}
	merged := string(mergedBytes)

	// 4) Apply env/host/namespace replacements (same semantics as before)
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
