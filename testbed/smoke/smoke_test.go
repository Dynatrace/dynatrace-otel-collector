package smoke

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
)

func Test_CollectorStarts(t *testing.T) {
	os.Chmod("../../build/otelcol-dynatrace", os.ModePerm)
	os.Mkdir("../../bin", os.ModePerm+os.ModePerm)

	abs, err := filepath.Abs("../../build/otelcol-dynatrace")
	require.NoError(t, err)

	// The testbed runner doesn't currently allow configuring the binary path.
	os.Symlink(abs, "../../bin/oteltestbedcol_linux_amd64")

	col := testbed.NewChildProcessCollector()

	cfg, err := os.ReadFile("../testdata/config-smoke.yaml")
	require.NoError(t, err)

	col.PrepareConfig(string(cfg))

	err = col.Start(testbed.StartParams{
		Name:        "otelcol-dynatrace",
		LogFilePath: "col.log",
	})
	require.NoError(t, err)

	resp, err := http.Get("http://localhost:9090/metrics")
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "otelcol_process_uptime")

	stopped, _ := col.Stop()
	require.True(t, stopped)

	os.RemoveAll("../../bin")
}
