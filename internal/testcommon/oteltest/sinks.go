package oteltest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type ExpectedValueMode int

const (
	AttributeMatchTypeEqual ExpectedValueMode = iota
	AttributeMatchTypeRegex
	AttributeMatchTypeExist
	UidRe = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}"
	IPRe  = "((25[0-5]|(2[0-4]|1\\d|[1-9]|)\\d)\\.?){4}"

	ServiceNameAttribute = "service.name"
)

type ExpectedValue struct {
	Mode  ExpectedValueMode
	Value string
}

func NewExpectedValue(mode ExpectedValueMode, value string) ExpectedValue {
	return ExpectedValue{
		Mode:  mode,
		Value: value,
	}
}

type ReceiverSinks struct {
	Metrics *MetricSinkConfig
	Traces  *TraceSinkConfig
	Logs    *LogSinkConfig
}

type MetricSinkConfig struct {
	Ports    *ReceiverPorts
	Consumer *consumertest.MetricsSink
}

type TraceSinkConfig struct {
	Ports    *ReceiverPorts
	Consumer *consumertest.TracesSink
}

type LogSinkConfig struct {
	Ports    *ReceiverPorts
	Consumer *consumertest.LogsSink
}

type ReceiverPorts struct {
	Grpc int
	Http int
}

func StartUpSinks(t *testing.T, sinks ReceiverSinks) func() {
	shutDownFuncs := []func(){}

	if sinks.Metrics != nil {
		fMetric := otlpreceiver.NewFactory()
		cfg := fMetric.CreateDefaultConfig().(*otlpreceiver.Config)
		setupReceiverPorts(cfg, sinks.Metrics.Ports)
		metricsRcvr, err := fMetric.CreateMetrics(context.Background(), receivertest.NewNopSettings(component.MustNewType("otlp")), cfg, sinks.Metrics.Consumer)
		require.NoError(t, err, "failed creating metrics receiver")
		require.NoError(t, metricsRcvr.Start(context.Background(), componenttest.NewNopHost()))
		shutDownFuncs = append(shutDownFuncs, func() {
			assert.NoError(t, metricsRcvr.Shutdown(context.Background()))
		})
	}
	if sinks.Traces != nil {
		fTrace := otlpreceiver.NewFactory()
		cfg := fTrace.CreateDefaultConfig().(*otlpreceiver.Config)
		setupReceiverPorts(cfg, sinks.Traces.Ports)
		tracesRcvr, err := fTrace.CreateTraces(context.Background(), receivertest.NewNopSettings(component.MustNewType("otlp")), cfg, sinks.Traces.Consumer)
		require.NoError(t, err, "failed creating traces receiver")
		require.NoError(t, tracesRcvr.Start(context.Background(), componenttest.NewNopHost()))
		shutDownFuncs = append(shutDownFuncs, func() {
			assert.NoError(t, tracesRcvr.Shutdown(context.Background()))
		})
	}
	if sinks.Logs != nil {
		fLog := otlpreceiver.NewFactory()
		cfg := fLog.CreateDefaultConfig().(*otlpreceiver.Config)
		setupReceiverPorts(cfg, sinks.Logs.Ports)
		logsRcvr, err := fLog.CreateLogs(context.Background(), receivertest.NewNopSettings(component.MustNewType("otlp")), cfg, sinks.Logs.Consumer)
		require.NoError(t, err, "failed creating logs receiver")
		require.NoError(t, logsRcvr.Start(context.Background(), componenttest.NewNopHost()))
		shutDownFuncs = append(shutDownFuncs, func() {
			assert.NoError(t, logsRcvr.Shutdown(context.Background()))
		})
	}

	return func() {
		for _, f := range shutDownFuncs {
			f()
		}
	}
}

func setupReceiverPorts(cfg *otlpreceiver.Config, ports *ReceiverPorts) {
	if ports != nil {
		cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:" + strconv.Itoa(ports.Grpc)
		cfg.HTTP.Endpoint = "0.0.0.0:" + strconv.Itoa(ports.Http)
	} else {
		cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"
		cfg.HTTP.Endpoint = "0.0.0.0:4318"
	}
}

func WaitForMetrics(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 5
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) >= entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}

func WaitForTraces(t *testing.T, entriesNum int, tc *consumertest.TracesSink) {
	timeoutMinutes := 5
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %d minutes", entriesNum,
		len(tc.AllTraces()), timeoutMinutes)
}

func ScanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string, kvs map[string]ExpectedValue, scopeSpanAttrs []map[string]ExpectedValue) {
	for i := 0; i < len(ts.AllTraces()); i++ {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get(ServiceNameAttribute)
			assert.True(t, exist, "Resource does not have the 'service.name' attribute")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, assertExpectedAttributes(resource.Attributes(), kvs))

			if len(scopeSpanAttrs) == 0 {
				return
			}

			assert.NotZero(t, traces.ResourceSpans().At(i).ScopeSpans().Len())
			assert.NotZero(t, traces.ResourceSpans().At(i).ScopeSpans().At(0).Spans().Len())

			scopeSpan := traces.ResourceSpans().At(i).ScopeSpans().At(0)

			// look for matching spans containing the desired attributes
			for _, spanAttrs := range scopeSpanAttrs {
				var err error
				for j := 0; j < scopeSpan.Spans().Len(); j++ {
					err = assertExpectedAttributes(scopeSpan.Spans().At(j).Attributes(), spanAttrs)
					if err == nil {
						break
					}
				}
				assert.NoError(t, err)
			}

			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func assertExpectedAttributes(attrs pcommon.Map, kvs map[string]ExpectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	attrs.Range(
		func(k string, v pcommon.Value) bool {
			if val, ok := kvs[k]; ok {
				switch val.Mode {
				case AttributeMatchTypeEqual:
					if val.Value == v.AsString() {
						foundAttrs[k] = true
					}
				case AttributeMatchTypeRegex:
					matched, _ := regexp.MatchString(val.Value, v.AsString())
					if matched {
						foundAttrs[k] = true
					}
				case AttributeMatchTypeExist:
					foundAttrs[k] = true
				}
			}
			return true
		},
	)

	var err error
	for k, v := range foundAttrs {
		if !v {
			err = errors.Join(err, fmt.Errorf("attribute '%v' not found", k))
		}
	}
	if err != nil {
		// if something is missing, add a summary with an overview of the expected and actual attributes for easier troubleshooting
		expectedJson, _ := json.MarshalIndent(kvs, "", "  ")
		actualJson, _ := json.MarshalIndent(attrs.AsRaw(), "", "  ")
		err = errors.Join(err, fmt.Errorf("one or more attributes were not found.\nExpected attributes:\n %s \nActual attributes: \n%s", expectedJson, actualJson))
	}
	return err
}

// ScanForServiceMetrics asserts that the metrics sink provided in the arguments
// contains the given metrics for a service
func ScanForServiceMetrics(t *testing.T, ms *consumertest.MetricsSink, expectedService string, expectedMetrics []string) {
	for _, r := range ms.AllMetrics() {
		for i := 0; i < r.ResourceMetrics().Len(); i++ {
			resource := r.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get(ServiceNameAttribute)
			assert.Equal(t, true, exist, "resource does not have the 'service.name' attribute")
			if service.AsString() != expectedService {
				continue
			}

			sm := r.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
			assert.NoError(t, assertExpectedMetrics(expectedMetrics, sm))
			return
		}
	}
	t.Fatalf("no metric found for service %s", expectedService)
}

func assertExpectedMetrics(expectedMetrics []string, sm pmetric.MetricSlice) error {
	var actualMetrics []string
	for i := 0; i < sm.Len(); i++ {
		actualMetrics = append(actualMetrics, sm.At(i).Name())
	}

	for _, m := range expectedMetrics {
		if !slices.Contains(actualMetrics, m) {
			return fmt.Errorf("metric: %s not found", m)
		}
	}
	return nil
}
