package integration

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// statsdDataSender implements TraceDataSender for Statsd http exporter.
type statsdDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

var _ testbed.MetricDataSender = (*statsdDataSender)(nil)

// NewStatsdDataSender creates a new client sender that will send
// to the specified port after Start is called.
func NewStatsdDataSender(host string, port int) testbed.MetricDataSender {
	return &statsdDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
}

func (zs *statsdDataSender) Start() error {
	params := exportertest.NewNopSettings()
	params.Logger = zap.L()
	cfg := statsdConfig{}

	s := &statsdSender{
		Host: zs.Host,
		Port: zs.Port,
	}

	exp, err := exporterhelper.NewMetricsExporter(context.TODO(), params, cfg, s.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}))
	if err != nil {
		return err
	}

	zs.Metrics = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (s *statsdSender) pushMetrics(ctx context.Context, td pmetric.Metrics) error {
	messages := convertMetric(td)
	// Resolve the UDP address
	udpAddr, err := net.ResolveUDPAddr("udp4", s.Host+":"+fmt.Sprint(s.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

	time.Sleep(5 * time.Second)

	// Create a UDP connection
	conn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP: %v", err)
	}
	defer conn.Close()

	// Set a write timeout
	err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return fmt.Errorf("failed to set write deadline: %v", err)
	}

	// Send the messages
	for _, message := range messages {
		_, err = conn.Write([]byte(message))
		if err != nil {
			return fmt.Errorf("failed to write to UDP connection: %v", err)
		}
	}

	return nil
}

type statsdSender struct {
	Host string
	Port int
}

func (s *statsdSender) shutdown(context.Context) error {
	return nil
}

func (s *statsdSender) start(ctx context.Context, host component.Host) error {
	return nil
}

type statsdConfig struct {
}

var _ component.Config = (*statsdConfig)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *statsdConfig) Validate() error {
	return nil
}

func convertMetric(m pmetric.Metrics) []string {
	res := []string{"my.gauge:42|g|#key:value"}
	//for i := 0; i < m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len(); i++ {
	// str := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i).Name() + ":" +
	// string(m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i).Gauge().DataPoints().At(0).IntValue()) + "|g|#" +
	// m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i).Gauge().DataPoints().At(0).Attributes().Get()

	//}
	return res
}

func (zs *statsdDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  statsd:
    endpoint: %s`, zs.GetEndpoint())
}

func (zs *statsdDataSender) ProtocolName() string {
	return "udp"
}
