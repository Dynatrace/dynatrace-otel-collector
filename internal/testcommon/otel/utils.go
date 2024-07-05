package otel

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
	"regexp"
	"testing"
	"time"
)

type ExpectedValueMode int

const (
	AttributeMatchTypeEqual ExpectedValueMode = iota
	AttributeMatchTypeRegex
	AttributeMatchTypeExist
	UidRe = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}"
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
	Metrics *consumertest.MetricsSink
	Traces  *consumertest.TracesSink
	Logs    *consumertest.LogsSink
}

func StartUpSinks(t *testing.T, sinks ReceiverSinks) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"
	cfg.HTTP.Endpoint = "0.0.0.0:4318"

	shutDownFuncs := []func(){}

	if sinks.Metrics != nil {
		metricsRcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sinks.Metrics)
		require.NoError(t, err, "failed creating metrics receiver")
		require.NoError(t, metricsRcvr.Start(context.Background(), componenttest.NewNopHost()))
		shutDownFuncs = append(shutDownFuncs, func() {
			assert.NoError(t, metricsRcvr.Shutdown(context.Background()))
		})
	}
	if sinks.Traces != nil {
		tracesRcvr, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sinks.Traces)
		require.NoError(t, err, "failed creating metrics receiver")
		require.NoError(t, tracesRcvr.Start(context.Background(), componenttest.NewNopHost()))
		shutDownFuncs = append(shutDownFuncs, func() {
			assert.NoError(t, tracesRcvr.Shutdown(context.Background()))
		})
	}
	if sinks.Logs != nil {
		logsRcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sinks.Logs)
		require.NoError(t, err, "failed creating metrics receiver")
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

func WaitForMetrics(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 5
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
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
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "Resource does not have the 'service.name' attribute")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, attributesContainValues(resource.Attributes(), kvs))
			assert.NotZero(t, traces.ResourceSpans().At(i).ScopeSpans().Len())
			assert.NotZero(t, traces.ResourceSpans().At(i).ScopeSpans().At(0).Spans().Len())

			scopeSpan := traces.ResourceSpans().At(i).ScopeSpans().At(0)

			// look for matching spans containing the desired attributes
			for _, spanAttrs := range scopeSpanAttrs {
				var err error
				for j := 0; j < scopeSpan.Spans().Len(); j++ {
					err = attributesContainValues(scopeSpan.Spans().At(j).Attributes(), spanAttrs)
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

func attributesContainValues(attrs pcommon.Map, kvs map[string]ExpectedValue) error {
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
			err = multierr.Append(err, fmt.Errorf("%v attribute not found. expected attributes: %#v. actual attributes: %#v", k, kvs, attrs.AsRaw()))
		}
	}
	return err
}
