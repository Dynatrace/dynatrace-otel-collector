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

var execPath = "../../bin/dynatrace-otel-collector"

func TestCollectorStarts(t *testing.T) {
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(execPath))

	cfg, err := os.ReadFile("../testdata/config-smoke.yaml")
	require.NoError(t, err)

	col.PrepareConfig(string(cfg))

	err = col.Start(testbed.StartParams{
		Name:        "dynatrace-otel-collector",
		LogFilePath: "col.log",
	})
	require.NoError(t, err)

	var resp *http.Response
	require.Eventually(t, func() bool {
		resp, err = http.Get("http://localhost:9090/metrics")

		return err == nil
	}, 3*time.Second, 1*time.Second)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "otelcol_process_uptime")

	stopped, _ := col.Stop()
	require.True(t, stopped)
}

type componentsOutput struct {
	BuildInfo  component.BuildInfo
	Receivers  []component.Type
	Processors []component.Type
	Exporters  []component.Type
	Connectors []component.Type
	Extensions []component.Type
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

func TestCollectorIsBuiltFromManifest(t *testing.T) {
	cmd := exec.Command(execPath, "components")
	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	err := cmd.Run()
	require.NoError(t, err)

	output, _ := io.ReadAll(&stdout)
	components := componentsOutput{}
	err = yaml.Unmarshal(output, &components)
	require.NoError(t, err)

	b, err := os.ReadFile("../../manifest.yaml")
	require.NoError(t, err)
	manifestComponents := manifest{}
	err = yaml.Unmarshal(b, &manifestComponents)
	require.NoError(t, err)

	assert.Equal(t, len(components.Connectors), len(manifestComponents.Connectors))
	assert.Equal(t, len(components.Exporters), len(manifestComponents.Exporters))
	assert.Equal(t, len(components.Extensions), len(manifestComponents.Extensions))
	assert.Equal(t, len(components.Processors), len(manifestComponents.Processors))
	assert.Equal(t, len(components.Receivers), len(manifestComponents.Receivers))
}
