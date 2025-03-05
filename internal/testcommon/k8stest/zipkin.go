package k8stest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ZipkinAppObjInfo struct {
	Namespace         string
	PodLabelSelectors map[string]any
}

type ZipkinAppCreateOpts struct {
	TestID       string
	ManifestsDir string
	OtlpEndpoint string
}

func CreateZipkinAppObjects(t *testing.T, client *otelk8stest.K8sClient, createOpts *ZipkinAppCreateOpts) ([]*unstructured.Unstructured, []*ZipkinAppObjInfo) {
	telemetrygenObjInfos := make([]*ZipkinAppObjInfo, 0)
	manifestFiles, err := os.ReadDir(createOpts.ManifestsDir)
	require.NoErrorf(t, err, "failed to read telemetrygen manifests directory %s", createOpts.ManifestsDir)
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(createOpts.ManifestsDir, manifestFile.Name())))
		manifest := &bytes.Buffer{}
		require.NoError(t, tmpl.Execute(manifest, map[string]string{
			"Name":         "zipkin-" + createOpts.TestID,
			"OTLPEndpoint": createOpts.OtlpEndpoint,
			"TestID":       createOpts.TestID,
		}))
		obj, err := otelk8stest.CreateObject(client, manifest.Bytes())
		require.NoErrorf(t, err, "failed to create zipkin object from manifest %s", manifestFile.Name())
		selector := obj.Object["spec"].(map[string]any)["selector"]
		telemetrygenObjInfos = append(telemetrygenObjInfos, &ZipkinAppObjInfo{
			Namespace:         obj.GetNamespace(),
			PodLabelSelectors: selector.(map[string]any)["matchLabels"].(map[string]any),
		})
		createdObjs = append(createdObjs, obj)
	}
	return createdObjs, telemetrygenObjInfos
}

func WaitForZipkinAppToStart(t *testing.T, client *otelk8stest.K8sClient, podNamespace string, podLabels map[string]any) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	listOptions := metav1.ListOptions{LabelSelector: otelk8stest.SelectorFromMap(podLabels).String()}
	podTimeoutMinutes := 3
	var podPhase string
	require.Eventually(t, func() bool {
		list, err := client.DynamicClient.Resource(podGVR).Namespace(podNamespace).List(context.Background(), listOptions)
		require.NoError(t, err, "failed to list zipkin example pods")
		if len(list.Items) == 0 {
			return false
		}
		podPhase = list.Items[0].Object["status"].(map[string]any)["phase"].(string)
		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 50*time.Millisecond,
		"zipkin example pods haven't started within %d minutes, latest pod phase is %s", podTimeoutMinutes, podPhase)
}
