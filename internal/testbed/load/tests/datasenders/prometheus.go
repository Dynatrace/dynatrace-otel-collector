package datasenders

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"net"
	"strings"
)

type multiHostPrometheusDataSender struct {
	dataSenders []testbed.MetricDataSender
	consumer.Metrics
}

func NewMultiHostPrometheusDataSender(host string, ports []int) testbed.MetricDataSender {
	ds := &multiHostPrometheusDataSender{
		dataSenders: make([]testbed.MetricDataSender, len(ports)),
	}

	for i, port := range ports {
		ds.dataSenders[i] = datasenders.NewPrometheusDataSender(host, port)
	}

	return ds
}

func (m multiHostPrometheusDataSender) Start() error {
	for _, prom := range m.dataSenders {
		if err := prom.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (m multiHostPrometheusDataSender) Flush() {
	for _, prom := range m.dataSenders {
		prom.Flush()
	}
}

func (m multiHostPrometheusDataSender) GetEndpoint() net.Addr {
	return nil
}

func (m multiHostPrometheusDataSender) GenConfigYAMLStr() string {
	yamlStr := ""

	for i, prom := range m.dataSenders {
		yamlStr += getIndexedPrometheusReceiverConfig(prom.GenConfigYAMLStr(), i)
	}

	return yamlStr
}

func (m multiHostPrometheusDataSender) ProtocolName() string {
	protocols := ""
	for i, prom := range m.dataSenders {
		if i < len(m.dataSenders)-1 {
			protocols += fmt.Sprintf("%s/%d,", prom.ProtocolName(), i)
		} else {
			protocols += fmt.Sprintf("%s/%d", prom.ProtocolName(), i)
		}
	}
	return protocols
}

func (m multiHostPrometheusDataSender) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for _, prom := range m.dataSenders {
		if err := prom.ConsumeMetrics(ctx, md); err != nil {
			return err
		}
	}
	return nil
}

func getIndexedPrometheusReceiverConfig(config string, i int) string {
	config = strings.Replace(config, "prometheus:", fmt.Sprintf("prometheus/%d:", i), 1)
	config = strings.Replace(config, "- job_name: 'testbed'", fmt.Sprintf("- job_name: 'testbed-%d'", i), 1)
	return config
}
