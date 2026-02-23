// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ConfigTemplate struct {
	Host      string
	Namespace string
	Templates []string
}

func GetCollectorConfig(path string, template ConfigTemplate) (string, error) {
	if path == "" {
		return "", nil
	}
	cfg, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	parsedConfig := string(cfg)

	replacerSlice := []string{
		"${env:DT_ENDPOINT}",
		fmt.Sprintf("http://%s:4318", template.Host),
		"${env:DT_API_TOKEN}",
		"",
		"${env:API_TOKEN}",
		"",
		"${env:NAMESPACE}",
		template.Namespace,
	}
	replacerSlice = append(replacerSlice, template.Templates...)

	r := strings.NewReplacer(
		replacerSlice...,
	)
	parsedConfig = r.Replace(parsedConfig)

	res := ""
	// prepend two tabs to each line to enable embedding the content in a k8s ConfigMap
	for _, line := range strings.Split(strings.TrimSuffix(parsedConfig, "\n"), "\n") {
		res += fmt.Sprintf("    %s\n", line)
	}

	return res, nil
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

func CreateCollectorObjects(t *testing.T, client *xk8stest.K8sClient, testID string, manifestsDir string, values map[string]string, host string, testNs string) []*unstructured.Unstructured {
	objs := xk8stest.CreateCollectorObjects(t, client, testID, manifestsDir, values, host)
	for _, o := range objs {
		if o.GetKind() == "Deployment" || o.GetKind() == "DaemonSet" || o.GetKind() == "StatefulSet" {
			RequireCollectorSecurityContextHardened(t, client, o)
			break
		}
	}

	return objs
}

// Helper to read overlay file content as string
func MustRead(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	require.NoErrorf(t, err, "read file %s", p)
	return string(b)
}

func RequireCollectorSecurityContextHardened(t *testing.T, k8sClient *xk8stest.K8sClient, workload *unstructured.Unstructured) {
	t.Helper()

	ns := workload.GetNamespace()

	spec := workload.Object["spec"].(map[string]any)
	selector := spec["selector"].(map[string]any)
	matchLabels := selector["matchLabels"].(map[string]any)

	parts := make([]string, 0, len(matchLabels))
	for k, v := range matchLabels {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	labelSelector := strings.Join(parts, ",")

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := k8sClient.DynamicClient.Resource(podGVR).Namespace(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	require.NoError(t, err)
	require.NotEmpty(t, list.Items, "pod not found: ns=%s selector=%q", ns, labelSelector)

	// then validate cont
	pod := &list.Items[0]
	containers, found, err := unstructured.NestedSlice(pod.Object, "spec", "containers")
	require.NoError(t, err)
	require.True(t, found, "pod missing spec.containers")
	require.NotEmpty(t, containers, "pod has no containers")

	// Use first container or match by name if you prefer.
	c0, ok := containers[0].(map[string]any)
	require.True(t, ok, "container[0] is not an object")

	sc, ok := c0["securityContext"].(map[string]any)
	require.True(t, ok, "collector container missing securityContext")

	// Assertions (unstructured numbers often come as float64)
	require.Equal(t, true, sc["readOnlyRootFilesystem"])
	require.Equal(t, false, sc["allowPrivilegeEscalation"])
	require.Equal(t, true, sc["runAsNonRoot"])
	require.EqualValues(t, float64(10001), sc["runAsUser"])
	require.EqualValues(t, float64(10001), sc["runAsGroup"])
	require.Equal(t, false, sc["privileged"])

	seccomp, ok := sc["seccompProfile"].(map[string]any)
	require.True(t, ok, "missing seccompProfile")
	require.Equal(t, "RuntimeDefault", seccomp["type"])

	caps, ok := sc["capabilities"].(map[string]any)
	require.True(t, ok, "missing capabilities")
	drop, ok := caps["drop"].([]any)
	require.True(t, ok, "missing capabilities.drop")
	require.Contains(t, drop, "all")
}
