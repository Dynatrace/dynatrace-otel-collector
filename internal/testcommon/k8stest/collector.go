// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/k8stest"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateCollectorObjects(t *testing.T, client *otelk8stest.K8sClient, testID string, manifestsDir string, collectorConfigPath string) []*unstructured.Unstructured {
	if manifestsDir == "" {
		manifestsDir = filepath.Join(".", "testdata", "e2e", "collector")
	}
	manifestFiles, err := os.ReadDir(manifestsDir)
	require.NoErrorf(t, err, "failed to read collector manifests directory %s", manifestsDir)
	host := otelk8stest.HostEndpoint(t)
	var podNamespace string
	var podLabels map[string]any
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	t.Log("Creating Collector objects...")

	collectorConfig, err := GetCollectorConfig(collectorConfigPath, host)
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)

	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(manifestsDir, manifestFile.Name())))
		manifest := &bytes.Buffer{}
		require.NoError(t, tmpl.Execute(manifest, map[string]string{
			"Name":              "otelcol-" + testID,
			"HostEndpoint":      host,
			"TestID":            testID,
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfig,
		}))
		obj, err := otelk8stest.CreateObject(client, manifest.Bytes())
		require.NoErrorf(t, err, "failed to create collector object from manifest %s", manifestFile.Name())
		objKind := obj.GetKind()
		if objKind == "Deployment" || objKind == "DaemonSet" {
			podNamespace = obj.GetNamespace()
			selector := obj.Object["spec"].(map[string]any)["selector"]
			podLabels = selector.(map[string]any)["matchLabels"].(map[string]any)
		}
		createdObjs = append(createdObjs, obj)
	}

	otelk8stest.WaitForCollectorToStart(t, client, podNamespace, podLabels)

	return createdObjs
}

// collectorConfig, err := getCollectorConfig(collectorConfigPath, host)
func GetCollectorConfig(path, host string) (string, error) {
	if path == "" {
		return "", nil
	}
	cfg, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	parsedConfig := string(cfg)

	r := strings.NewReplacer(
		"${env:DT_ENDPOINT}",
		fmt.Sprintf("http://%s:4318", host),
		"${env:DT_API_TOKEN}",
		"",
		"${env:API_TOKEN}",
		"",
	)
	parsedConfig = r.Replace(parsedConfig)

	res := ""
	// prepend two tabs to each line to enable embedding the content in a k8s ConfigMap
	for _, line := range strings.Split(strings.TrimSuffix(parsedConfig, "\n"), "\n") {
		res += fmt.Sprintf("    %s\n", line)
	}

	return res, nil
}
