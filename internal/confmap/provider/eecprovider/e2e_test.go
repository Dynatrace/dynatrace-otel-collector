//go:build e2e

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider/eecprovider"

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const CollectorBinary = "../../../../bin/dynatrace-otel-collector"

func TestGetsConfig(t *testing.T) {
	f, err := os.ReadFile("./testdata/otel-config.yaml")
	require.NoError(t, err)
	fs := &fileserver{config: f}
	ts := httptest.NewServer(http.HandlerFunc(fs.HandleRequest))
	cmd := exec.Command(CollectorBinary, fmt.Sprintf("--config=%s", configureProvider(t, ts.URL)))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		err = cmd.Process.Signal(os.Interrupt)
		require.NoError(t, err)

		err = cmd.Wait()
		require.NoError(t, err)
		ts.Close()
	})

	require.Eventually(t, func() bool {
		resp, err := http.DefaultClient.Get("http://localhost:55679/debug/servicez")
		if err != nil {
			return false
		}
		resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, time.Second*3, time.Millisecond*50)
}

func TestReloadsConfig(t *testing.T) {
	f, err := os.ReadFile("./testdata/otel-config.yaml")
	require.NoError(t, err)
	fs := &fileserver{config: f}
	ts := httptest.NewServer(http.HandlerFunc(fs.HandleRequest))
	cmd := exec.Command(CollectorBinary, fmt.Sprintf("--config=%s", configureProvider(t, ts.URL)))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		err = cmd.Process.Signal(os.Interrupt)
		require.NoError(t, err)

		err = cmd.Wait()
		require.NoError(t, err)
		ts.Close()
	})

	require.Eventually(t, func() bool {
		resp, err := http.DefaultClient.Get("http://localhost:55679/debug/servicez")
		if err != nil {
			return false
		}
		resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, time.Second*3, time.Millisecond*50)

	fs.config, err = os.ReadFile("./testdata/otel-config-updated.yaml")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		// Test that the zpages extension is running on the new port.
		resp, err := http.DefaultClient.Get("http://localhost:55680/debug/servicez")
		if err != nil {
			return false
		}
		resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, time.Second*3, time.Millisecond*50)
}

// This behavior should likely change in the future, but this test ensures
// that we know when the behavior changes.
func TestFailsOnBadConfig(t *testing.T) {
	fs := &fileserver{}
	ts := httptest.NewServer(http.HandlerFunc(fs.HandleRequest))
	cmd := exec.Command(CollectorBinary, fmt.Sprintf("--config=%s", configureProvider(t, ts.URL)))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	require.NoError(t, err)

	err = cmd.Wait()
	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError)
	ts.Close()
}

type fileserver struct {
	config []byte
}

func (fs *fileserver) HandleRequest(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(fs.config)
	if err != nil {
		fmt.Println("Write failed: ", err)
	}
}

func configureProvider(t *testing.T, URL string) string {
	parsedURL, err := url.Parse(URL)
	require.NoError(t, err)
	parsedURL.Scheme = "eec"
	params, err := url.ParseQuery(parsedURL.Fragment)
	require.NoError(t, err)
	params.Set(Insecure, "true")
	params.Set(RefreshInterval, "10ms")
	parsedURL.Fragment = params.Encode()
	return parsedURL.String()
}
