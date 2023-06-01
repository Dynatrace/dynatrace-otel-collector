package smoke

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	os.Symlink(abs, "../../bin/oteltestbedcol_linux_amd64")
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

	exceptions := map[component.Type]component.Type{
		"memory_ballast": "ballast",
		"memory_limiter": "memorylimiter",
	}

	for i, con := range components.Connectors {
		assert.Contains(t, manifestComponents.Connectors[i].Gomod, "/"+string(con)+"connector")
	}
	for i, ext := range components.Extensions {
		name := ext
		if val, ok := exceptions[ext]; ok {
			name = val
		}
		assert.Contains(t, manifestComponents.Extensions[i].Gomod, "/"+string(name)+"extension")
	}
	for i, prs := range components.Processors {
		name := prs
		if val, ok := exceptions[prs]; ok {
			name = val
		}
		assert.Contains(t, manifestComponents.Processors[i].Gomod, "/"+string(name)+"processor")
	}
	for i, rcv := range components.Receivers {
		assert.Contains(t, manifestComponents.Receivers[i].Gomod, "/"+string(rcv)+"receiver")
	}
	for i, exp := range components.Exporters {
		assert.Contains(t, manifestComponents.Exporters[i].Gomod, "/"+string(exp)+"exporter")
	}
}
