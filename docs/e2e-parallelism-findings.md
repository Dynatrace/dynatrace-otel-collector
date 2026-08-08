# E2E Parallelism Findings — 2026-07-29

## Current state

`make e2e` runs all 19 suites sequentially. Measured wall-clock times from a
single local run (Apple M-series, 16 GB Docker RAM, Kind v0.32.0, k8s v1.36.1):

| Suite | Time |
|---|---|
| kafka | 318s | 🐢 outlier |
| prometheus-large-scale | 140s | |
| zipkin | 119s | |
| k8scluster | 115s | |
| filestorage | 114s | |
| kubeletstats | 106s | |
| k8senrichment | 102s | |
| self-monitoring | 102s | |
| self-monitoring-prometheus | 84s | |
| loadbalancing | 84s | |
| bearertokenauth | 77s | |
| statsd | 76s | |
| k8scombined | 75s | |
| k8sobjects | 64s | |
| redaction | 50s | |
| netflow | 48s | |
| resource-detection | 45s | |
| hostmetrics | 40s | |
| prometheus | 38s | |
| genainormalizer | 28s | |

Total sequential: **~1,975s (~33 minutes).**
kafka alone is 318s — nearly 5× the median and a candidate for targeted speedup.

## Blocker: port conflicts

Every test binds OTLP sink receivers to fixed ports on the test host.
Running two suites concurrently would cause `bind: address already in use`.

Port map (suites grouped by conflict):

| Port(s) | Suites that claim it |
|---|---|
| 4317 / 4318 (default) | k8senrichment, prometheus, zipkin, statsd, redaction, bearertokenauth, resource-detection, netflow, k8sobjects, kubeletstats, k8scluster, filestorage (12 suites) |
| 4319–4322 | self-monitoring, kafka, k8scombined |
| 4320 | self-monitoring-prometheus, hostmetrics |
| 4327–4332 | loadbalancing, prometheus-large-scale, combinedload |
| 5317 | genainormalizer |

## Solution: dynamic port allocation

Replace every hardcoded port with OS-assigned free ports via a helper:

```go
// internal/testcommon/testutil/port.go
func FreePort(t *testing.T) int {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    port := ln.Addr().(*net.TCPAddr).Port
    require.NoError(t, ln.Close())
    return port
}
```

Each test calls `FreePort` before starting sinks and passes the result both to
`ReceiverPorts` and to the collector config overlay template (currently using
`fmt.Sprintf` with `%s` for host — add `%d` for each port).

### Scope of change

| Change | Count |
|---|---|
| New Go file (`testutil/port.go`) | 1 |
| Test files — swap hardcoded ports for `FreePort` calls | ~19 |
| Existing overlay YAMLs — add `%d` port format arg | ~7 |
| New overlay YAMLs — for the 12 suites that have none today | ~12 |
| **Total files touched** | **~39** |

`sinks.go` itself needs no changes — `ReceiverPorts` already accepts any int.

## Resource envelope for parallel runs

A single Kind control-plane node idles at ~600 MB RAM. Add the collector pod
(256 Mi limit) and system pods → ~850 MB per suite.

Docker RAM available on this machine: **15.6 GB**

| Concurrency | RAM needed | Feasible | Speedup |
|---|---|---|---|
| 4 | ~3.4 GB | ✅ | ~4× (~6 min) |
| 8 | ~6.8 GB | ✅ | ~7× (~3 min) |
| 12 | ~10 GB | ⚠️ tight | ~9× |
| 19 | ~16 GB | ❌ risky | ~15× |

**Recommended target: 8 concurrent** — fits comfortably, ~7× speedup.

After port conflicts are resolved, parallelism is a one-line `make -j 8` change
(plus making `E2E_CLUSTER_NAME` and `E2E_KUBECONFIG` suite-scoped in the
Makefile so clusters don't collide).
