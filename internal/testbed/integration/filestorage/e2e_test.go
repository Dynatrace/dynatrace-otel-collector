// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	waitFor  = 30 * time.Second
	waitTick = 200 * time.Millisecond
)

var entryRe = regexp.MustCompile(`Log entry (\d+) from filestorage test`)

// backend is the logs backend of the Collector. While it is unavailable it rejects requests,
// which the OTLP receiver turns into HTTP 503, and the Collector retries.
type backend struct {
	consumertest.LogsSink

	mu          sync.Mutex
	unavailable bool
	rejected    []plog.Logs
}

func (b *backend) setUnavailable(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unavailable = v
}

func (b *backend) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	b.mu.Lock()
	if b.unavailable {
		cp := plog.NewLogs()
		ld.CopyTo(cp)
		b.rejected = append(b.rejected, cp)
		b.mu.Unlock()
		return errors.New("backend unavailable")
	}
	b.mu.Unlock()
	return b.LogsSink.ConsumeLogs(ctx, ld)
}

func (b *backend) rejectedLogs() []plog.Logs {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]plog.Logs(nil), b.rejected...)
}

// collectorConfig returns the config_examples/filestorage.yaml config adapted for a local run,
// and the port of its health_check extension.
func collectorConfig(t *testing.T, logDir, storageDir string, sinkPort int) (string, int) {
	healthPort := testutil.GetAvailablePort(t)
	cfg := testutil.LoadConfigExample(t, "filestorage.yaml").
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		ReplaceString("/var/lib/otelcol/file_storage", storageDir).
		// The compaction directory defaults to a path under /var/lib/otelcol.
		ReplaceString("    compaction:\n", "    compaction:\n      directory: "+filepath.Join(storageDir, "compaction")+"\n").
		ReplaceString("/var/log/app/*.log", filepath.Join(logDir, "*.log")).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPort) }).
		Replace("internal metrics", testutil.DisableInternalMetrics)
	return cfg.String(), healthPort
}

// writeEntries appends the numbered lines from..to to the file at path.
func writeEntries(t *testing.T, path string, from, to int) {
	var s string
	for n := from; n <= to; n++ {
		s += fmt.Sprintf("Log entry %d from filestorage test\n", n)
	}
	appendLine(t, path, s)
}

func appendLine(t *testing.T, path, s string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(s)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// waitForPipeline waits until the file_log receiver is reading the file. It uses the default
// start_at: end, so lines written before its first poll are skipped, and the health check port
// opens before the receivers start.
func waitForPipeline(t *testing.T, path string, b *backend) {
	require.Eventually(t, func() bool {
		appendLine(t, path, "warm-up\n")
		return b.LogRecordCount() > 0
	}, waitFor, waitTick, "no log record reached the backend")
}

// entryCounts counts how often each numbered entry was received.
func entryCounts(logs []plog.Logs) map[int]int {
	counts := map[int]int{}
	for _, ld := range logs {
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rl := ld.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				lrs := rl.ScopeLogs().At(j).LogRecords()
				for k := 0; k < lrs.Len(); k++ {
					if m := entryRe.FindStringSubmatch(lrs.At(k).Body().AsString()); m != nil {
						n, _ := strconv.Atoi(m[1])
						counts[n]++
					}
				}
			}
		}
	}
	return counts
}

func requireAllReceived(t *testing.T, from, to int, logs func() []plog.Logs, msg string) {
	require.Eventually(t, func() bool {
		counts := entryCounts(logs())
		for n := from; n <= to; n++ {
			if counts[n] < 1 {
				return false
			}
		}
		return true
	}, waitFor, waitTick, msg)
}

// TestE2E_FileStorageCheckpoint tests that the file_log receiver resumes from the offset stored
// in file_storage after a Collector restart: lines written while the Collector is down are
// neither lost nor duplicated.
// The config under test is config_examples/filestorage.yaml.
func TestE2E_FileStorageCheckpoint(t *testing.T) {
	b := new(backend)
	ports := oteltest.StartReceiver(t, oteltest.Consumers{Logs: b})
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "app.log")
	cfg, healthPort := collectorConfig(t, logDir, filepath.Join(t.TempDir(), "file_storage"), ports.Http)

	c1 := testutil.StartCollector(t, testutil.CollectorParams{Name: "filestorage-1", Config: cfg, ReadyPort: healthPort})
	waitForPipeline(t, logPath, b)

	writeEntries(t, logPath, 1, 50)
	requireAllReceived(t, 1, 50, b.AllLogs, "entries 1..50 did not arrive")
	c1.Stop(t)

	writeEntries(t, logPath, 51, 100)

	testutil.StartCollector(t, testutil.CollectorParams{Name: "filestorage-2", Config: cfg, ReadyPort: healthPort})
	requireAllReceived(t, 51, 100, b.AllLogs, "entries 51..100 did not arrive after restart")

	// Lost lines mean the checkpoint wasn't used, duplicates mean the file was re-read.
	counts := entryCounts(b.AllLogs())
	for n := 1; n <= 100; n++ {
		require.Equalf(t, 1, counts[n], "entry %d was received %d times", n, counts[n])
	}
}

// TestE2E_FileStoragePersistentQueue tests that the persistent sending_queue of the exporter
// survives a Collector restart: lines that couldn't be delivered because the backend was down
// are delivered by the next Collector, even though the log file is gone.
// The config under test is config_examples/filestorage.yaml.
func TestE2E_FileStoragePersistentQueue(t *testing.T) {
	b := new(backend)
	ports := oteltest.StartReceiver(t, oteltest.Consumers{Logs: b})
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "app.log")
	cfg, healthPort := collectorConfig(t, logDir, filepath.Join(t.TempDir(), "file_storage"), ports.Http)

	c1 := testutil.StartCollector(t, testutil.CollectorParams{Name: "filestorage-1", Config: cfg, ReadyPort: healthPort})
	waitForPipeline(t, logPath, b)

	b.setUnavailable(true)
	writeEntries(t, logPath, 1, 50)
	// The lines were read, enqueued and dispatched at least once.
	requireAllReceived(t, 1, 50, b.rejectedLogs, "entries 1..50 were not dispatched to the backend")

	// Graceful shutdown interrupts the retry and leaves the items in the queue.
	c1.Stop(t)

	// From now on the lines can only come from the persistent queue, not from file_log.
	require.NoError(t, os.Remove(logPath))

	b.setUnavailable(false)
	testutil.StartCollector(t, testutil.CollectorParams{Name: "filestorage-2", Config: cfg, ReadyPort: healthPort})
	requireAllReceived(t, 1, 50, b.AllLogs, "entries 1..50 were not delivered after restart")

	counts := entryCounts(b.AllLogs())
	for n := 1; n <= 50; n++ {
		require.Equalf(t, 1, counts[n], "entry %d was received %d times", n, counts[n])
	}
}
