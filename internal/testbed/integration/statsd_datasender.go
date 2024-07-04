package integration

import (
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// statsdDataSender implements TraceDataSender for Statsd http exporter.
type statsdDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
	Messages []string
}

// NewStatsdDataSender creates a new client sender that will send
// to the specified port after Start is called.
func NewStatsdDataSender(host string, port int, messages []string) testbed.MetricDataSender {
	return &statsdDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
		Messages: messages,
	}
}

func (zs *statsdDataSender) Start() error {
	// Resolve the UDP address
	udpAddr, err := net.ResolveUDPAddr("udp4", zs.Host+":"+fmt.Sprint(zs.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

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
	for _, message := range zs.Messages {
		_, err = conn.Write([]byte(message))
		if err != nil {
			return fmt.Errorf("failed to write to UDP connection: %v", err)
		}
	}

	return nil
}

func (zs *statsdDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  statsd:
    endpoint: %s`, zs.GetEndpoint())
}

func (zs *statsdDataSender) ProtocolName() string {
	return "statsd"
}
