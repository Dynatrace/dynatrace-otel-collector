package smoke

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"gopkg.in/yaml.v3"
)

var outputDir = "../../build/otelcol-dynatrace"

func collectorSetup(t *testing.T) {
	os.Chmod(outputDir, os.ModePerm)
	os.Mkdir("../../bin", os.ModePerm+os.ModePerm)

	abs, err := filepath.Abs("../../build/otelcol-dynatrace")
	require.NoError(t, err)

	// The testbed runner doesn't currently allow configuring the binary path.
	os.Symlink(abs, fmt.Sprintf("../../bin/oteltestbedcol_%s_%s", runtime.GOOS, runtime.GOARCH))
}

func collectorTeardown() {
	os.RemoveAll("../../bin")
}

func TestCollectorStarts(t *testing.T) {
	collectorSetup(t)
	defer collectorTeardown()

	col := testbed.NewChildProcessCollector()

	cfg, err := os.ReadFile("../testdata/config-smoke.yaml")
	require.NoError(t, err)

	col.PrepareConfig(string(cfg))

	err = col.Start(testbed.StartParams{
		Name:        "otelcol-dynatrace",
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
	cmd := exec.Command(outputDir, "components")
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

	// Will assert on connectors starting in Collector version 0.79
	// https://github.com/open-telemetry/opentelemetry-collector/pull/7809
	assert.Equal(t, len(components.Exporters), len(manifestComponents.Exporters))
	assert.Equal(t, len(components.Extensions), len(manifestComponents.Extensions))
	assert.Equal(t, len(components.Processors), len(manifestComponents.Processors))
	assert.Equal(t, len(components.Receivers), len(manifestComponents.Receivers))
}
