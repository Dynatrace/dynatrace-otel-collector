// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"fmt"
	"os"
	"strings"
)

type ConfigTemplate struct {
	Host      string
	Namespace string
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

	r := strings.NewReplacer(
		"${env:DT_ENDPOINT}",
		fmt.Sprintf("http://%s:4318", template.Host),
		"${env:DT_API_TOKEN}",
		"",
		"${env:API_TOKEN}",
		"",
		"${env:NAMESPACE}",
		template.Namespace,
	)
	parsedConfig = r.Replace(parsedConfig)

	res := ""
	// prepend two tabs to each line to enable embedding the content in a k8s ConfigMap
	for _, line := range strings.Split(strings.TrimSuffix(parsedConfig, "\n"), "\n") {
		res += fmt.Sprintf("    %s\n", line)
	}

	return res, nil
}
