package smoke

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"gopkg.in/yaml.v3"
)

var execPath = "../../../bin/dynatrace-otel-collector"

var duplicateAliasExporters = []string{
	"otlp",
	"otlphttp",
}

var duplicateAliasProcessors = []string{
	"k8sattributes",
}

func TestCollectorStarts(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		preCheck   func(t *testing.T)
	}{
		{
			name:       "Basic test",
			configFile: "config-smoke.yaml",
		},
		{
			name:       "All components",
			configFile: "config-allcomponents.yaml",
			preCheck: func(t *testing.T) {
				components := getComponents(t)

				b, err := os.ReadFile("../testdata/config-allcomponents.yaml")
				require.NoError(t, err)
				testdataComponents := collectorConf{}
				err = yaml.Unmarshal(b, &testdataComponents)
				require.NoError(t, err)

				for _, c := range components.Receivers {
					_, ok := testdataComponents.Receivers[c.Name]
					require.True(t, ok, "config-allcomponents.yaml is missing receiver "+c.Name)
				}

				for _, c := range components.Processors {
					_, ok := testdataComponents.Processors[c.Name]
					require.True(t, ok, "config-allcomponents.yaml is missing processor "+c.Name)
				}

				for _, c := range components.Exporters {
					_, ok := testdataComponents.Exporters[c.Name]
					require.True(t, ok, "config-allcomponents.yaml is missing exporter "+c.Name)
				}

				for _, c := range components.Connectors {
					_, ok := testdataComponents.Connectors[c.Name]
					require.True(t, ok, "config-allcomponents.yaml is missing connector "+c.Name)
				}

				for _, c := range components.Extensions {
					_, ok := testdataComponents.Extensions[c.Name]
					require.True(t, ok, "config-allcomponents.yaml is missing extension "+c.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preCheck != nil {
				tt.preCheck(t)
			}

			col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(execPath))

			cfg, err := os.ReadFile("../testdata/" + tt.configFile)
			require.NoError(t, err)

			col.PrepareConfig(t, string(cfg))

			err = col.Start(testbed.StartParams{
				Name:        "dynatrace-otel-collector",
				LogFilePath: "col.log",
			})
			require.NoError(t, err)

			var resp *http.Response
			require.Eventually(t, func() bool {
				resp, err = http.Get("http://localhost:9090/metrics")

				return err == nil
			}, 15*time.Second, 1*time.Second)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), "otelcol_process_uptime")

			stopped, _ := col.Stop()
			require.True(t, stopped)
		})
	}
}

func TestCollectorIsBuiltFromManifest(t *testing.T) {
	components := getComponents(t)
	b, err := os.ReadFile("../../../manifest.yaml")
	require.NoError(t, err)
	manifestComponents := manifest{}
	err = yaml.Unmarshal(b, &manifestComponents)
	require.NoError(t, err)

	assert.Equal(t, len(components.Connectors), len(manifestComponents.Connectors))
	assert.Equal(t, len(components.Exporters), len(manifestComponents.Exporters)-len(duplicateAliasExporters))
	assert.Equal(t, len(components.Extensions), len(manifestComponents.Extensions))
	assert.Equal(t, len(components.Processors), len(manifestComponents.Processors)-len(duplicateAliasProcessors))
	assert.Equal(t, len(components.Receivers), len(manifestComponents.Receivers))
}

type componentMetadata struct {
	Name      string
	Stability map[string]string
}

type componentsOutput struct {
	BuildInfo  component.BuildInfo
	Receivers  []componentMetadata
	Processors []componentMetadata
	Exporters  []componentMetadata
	Connectors []componentMetadata
	Extensions []componentMetadata
}

type gomod struct {
	Gomod string
}

type manifest struct {
	Receivers  []gomod
	Processors []gomod
	Exporters  []gomod
	Connectors []gomod
	Extensions []gomod
}

type collectorConf struct {
	Receivers  map[string]any
	Processors map[string]any
	Exporters  map[string]any
	Connectors map[string]any
	Extensions map[string]any
}

func getComponents(t *testing.T) componentsOutput {
	cmd := exec.Command(execPath, "components")
	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	err := cmd.Run()
	require.NoError(t, err)

	output, _ := io.ReadAll(&stdout)
	components := componentsOutput{}
	err = yaml.Unmarshal(output, &components)
	require.NoError(t, err)

	return components
}
