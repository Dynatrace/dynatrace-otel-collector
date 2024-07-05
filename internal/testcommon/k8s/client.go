// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8s // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"errors"
	"fmt"
	"os"

	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	testKubeConfig   = "/tmp/kube-config-collector-e2e-testing"
	kubeConfigEnvVar = "KUBECONFIG"
)

type K8sClient struct {
	DynamicClient   *dynamic.DynamicClient
	DiscoveryClient *discovery.DiscoveryClient
	Mapper          *restmapper.DeferredDiscoveryRESTMapper
}

func NewK8sClient() (*K8sClient, error) {

	kubeconfigPath := testKubeConfig

	if kubeConfigFromEnv := os.Getenv(kubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	if kubeconfigPath == "" {
		return nil, errors.New("Please provide file path to load kubeconfig")
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load kubeconfig from %s: %w", kubeconfigPath, err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %w", err)
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	return &K8sClient{
		DynamicClient: dynamicClient, DiscoveryClient: discoveryClient, Mapper: mapper}, nil
}
