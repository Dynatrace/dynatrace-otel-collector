// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Paths relative to a package directory directly below internal/testbed/integration.
const (
	SubpackageCollectorExecPath = "../../../../bin/dynatrace-otel-collector"
	SubpackageConfigExamplesDir = "../../../../config_examples"
)

// Config is a Collector config that is adapted for a test. Every edit is required to change
// the config, so that changes to the original can't silently break the test.
type Config struct {
	t   testing.TB
	cfg string
}

// LoadConfig reads the config at path.
func LoadConfig(t testing.TB, path string) *Config {
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return &Config{t: t, cfg: string(raw)}
}

// LoadConfigExample reads a config from config_examples. It must be called from a package
// directly below internal/testbed/integration.
func LoadConfigExample(t testing.TB, name string) *Config {
	return LoadConfig(t, filepath.Join(SubpackageConfigExamplesDir, name))
}

// Replace applies f to the config. name describes the edit in the failure message.
func (c *Config) Replace(name string, f func(string) string) *Config {
	c.t.Helper()
	out := f(c.cfg)
	require.NotEqual(c.t, c.cfg, out, "config replacement %q had no effect", name)
	c.cfg = out
	return c
}

// ReplaceString replaces the first occurrence of old with new.
func (c *Config) ReplaceString(old, new string) *Config {
	c.t.Helper()
	return c.Replace(old, func(s string) string { return strings.Replace(s, old, new, 1) })
}

func (c *Config) String() string {
	return c.cfg
}

func ReplaceHealthCheckPort(cfg string, port int) string {
	return replaceEndpointPort(cfg, 13133, port)
}

// DisableInternalMetrics turns off the Collector's internal metrics server. It binds the fixed
// port 8888, which would clash with other Collectors on the machine (or a leftover from an aborted run).
func DisableInternalMetrics(cfg string) string {
	return strings.Replace(cfg, "service:\n", "service:\n  telemetry:\n    metrics:\n      level: none\n", 1)
}

// DisableSendingQueueAndRetry makes the results of the given exporter synchronous, so that
// export failures are returned to the client and show up immediately.
func DisableSendingQueueAndRetry(cfg, exporter string) string {
	header := "\n  " + exporter + ":\n"
	return strings.Replace(cfg, header,
		header+"    retry_on_failure:\n      enabled: false\n    sending_queue:\n      enabled: false\n", 1)
}

// CollectorParams configures a Collector started with StartCollector.
type CollectorParams struct {
	// Name identifies the Collector in the test output.
	Name   string
	Config string
	// ReadyPort is a port the Collector listens on once it has started, e.g. its OTLP receiver.
	ReadyPort int
	// ExecPath defaults to SubpackageCollectorExecPath.
	ExecPath string
	// Args are additional command line arguments, e.g. feature gates.
	Args []string
}

// StartCollector runs the Collector binary as a child process until the test ends. Its log
// is written to the test output if the test fails.
func StartCollector(t *testing.T, p CollectorParams) {
	t.Helper()
	execPath := p.ExecPath
	if execPath == "" {
		execPath = SubpackageCollectorExecPath
	}
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(execPath))
	cleanupCfg, err := col.PrepareConfig(t, p.Config)
	require.NoError(t, err)
	t.Cleanup(cleanupCfg)

	logPath := filepath.Join(t.TempDir(), "collector.log")
	require.NoError(t, col.Start(testbed.StartParams{
		Name:        p.Name,
		LogFilePath: logPath,
		CmdArgs:     p.Args,
	}))
	t.Cleanup(func() {
		_, err := col.Stop()
		assert.NoError(t, err)
		if t.Failed() {
			if b, err := os.ReadFile(logPath); err == nil {
				t.Logf("%s log:\n%s", p.Name, b)
			}
		}
	})

	WaitForListener(t, net.JoinHostPort("localhost", strconv.Itoa(p.ReadyPort)))
}

// WaitForListener waits until addr accepts TCP connections.
func WaitForListener(t testing.TB, addr string) {
	t.Helper()
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 30*time.Second, 200*time.Millisecond, "nothing started listening on %s", addr)
}
