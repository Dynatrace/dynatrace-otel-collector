// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryyaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
)

type OperationFunc func(client *K8sClient, manifest []byte) (*unstructured.Unstructured, error)

func CreateObject(client *K8sClient, manifest []byte) (*unstructured.Unstructured, error) {
	obj, gvk, err := ConvertBytesToUnstructured(manifest)
	if err != nil {
		return nil, err
	}
	gvr, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	var resource dynamic.ResourceInterface
	if gvr.Scope.Name() == meta.RESTScopeNameNamespace {
		resource = client.DynamicClient.Resource(gvr.Resource).Namespace(obj.GetNamespace())
	} else {
		// cluster-scoped resources
		resource = client.DynamicClient.Resource(gvr.Resource)
	}

	return resource.Create(context.Background(), obj, metav1.CreateOptions{})
}

func DeleteObject(client *K8sClient, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}
	var resource dynamic.ResourceInterface
	if gvr.Scope.Name() == meta.RESTScopeNameNamespace {
		resource = client.DynamicClient.Resource(gvr.Resource).Namespace(obj.GetNamespace())
	} else {
		// cluster-scoped resources
		resource = client.DynamicClient.Resource(gvr.Resource)
	}
	deletePolicy := metav1.DeletePropagationForeground
	return resource.Delete(context.Background(), obj.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
}

func DeleteObjectFromManifest(client *K8sClient, manifest []byte) (*unstructured.Unstructured, error) {
	unstruct, _, err := ConvertBytesToUnstructured(manifest)
	if err != nil {
		return nil, err
	}
	return nil, DeleteObject(client, unstruct)
}

func PerformOperationOnYAMLFiles(client *K8sClient, dirPath string, operation OperationFunc) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".yaml" || filepath.Ext(file.Name()) == ".yml" {
			filePath := filepath.Join(dirPath, file.Name())
			yamlData, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("error reading file: %v", err)
			}
			_, err = operation(client, yamlData)
			if err != nil {
				return fmt.Errorf("error performing operation on file %s: %v", filePath, err)
			}
		}
	}

	return nil
}

// WaitForDeploymentPods waits until all pods of the deployment are up and running
func WaitForDeploymentPods(dynamicClient dynamic.Interface, namespace, deploymentName string, timeout time.Duration) error {
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment %s pods to be ready", deploymentName)
		default:
			unstructuredDeployment, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting deployment %s: %v", deploymentName, err)
			}

			deployment, err := ConvertUnstructuredToDeployment(unstructuredDeployment)
			if err != nil {
				return fmt.Errorf("error converting unstructured to deployment: %v", err)
			}

			if int64(*deployment.Spec.Replicas) == int64(deployment.Status.ReadyReplicas) {
				return nil
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func GetPodNameByLabels(dynamicClient dynamic.Interface, namespace string, podLabels map[string]string) (string, error) {
	// Convert the map of labels to a selector
	labelSelector := labels.SelectorFromSet(podLabels).String()

	// Define the GVR (GroupVersionResource) for pods
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	// List pods in the specified namespace with the given labels
	pods, err := dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("error listing pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found with the specified labels")
	}

	// Return the name of the first pod that matches the labels
	return pods.Items[0].GetName(), nil
}

// ConvertUnstructuredToDeployment converts an unstructured.Unstructured object to a v1.Deployment
func ConvertUnstructuredToDeployment(obj *unstructured.Unstructured) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deployment)
	if err != nil {
		return nil, fmt.Errorf("error converting unstructured to deployment: %v", err)
	}
	return deployment, nil
}

// ConvertBytesToUnstructured converts a []byte to an unstructured.Unstructured object
func ConvertBytesToUnstructured(data []byte) (*unstructured.Unstructured, *schema.GroupVersionKind, error) {
	decoder := apimachineryyaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, gvk, err := decoder.Decode(data, nil, obj)
	if err != nil {
		return nil, nil, err
	}
	return obj, gvk, nil
}
