package k8stest

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	TestKubeConfig = "/tmp/kube-config-collector-e2e-testing"
)

func NewMetricsClientSet() (*metricsv.Clientset, error) {
	kubeconfigPath := TestKubeConfig

	if kubeconfigPath == "" {
		return nil, fmt.Errorf("please provide file path to load kubeconfig")
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load kubeconfig from %s: %w", kubeconfigPath, err)
	}

	// Create a clientset for the metrics API
	metricsClientset, err := metricsv.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating metrics clientset: %v", err)
	}

	return metricsClientset, nil
}

func FetchPodMetrics(metricsClientset *metricsv.Clientset, namespace, podName string) (string, string, error) {
	// Fetch pod metrics
	podMetrics, err := metricsClientset.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("error fetching pod metrics: %v", err)
	}

	var totalCPU, totalMemory resource.Quantity
	var containerCount int64

	for _, container := range podMetrics.Containers {
		totalCPU.Add(container.Usage["cpu"])
		totalMemory.Add(container.Usage["memory"])
		containerCount++
	}

	// Calculate averages
	cpuAvg := totalCPU.ScaledValue(resource.Milli)
	memAvg := totalMemory.ScaledValue(resource.Mega)

	return fmt.Sprintf("%dm", cpuAvg), fmt.Sprintf("%dMi", memAvg), nil
}
