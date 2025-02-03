package statsd

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	UDP  = "udp"
	UDP4 = "udp4"
	UDP6 = "udp6"
	TCP  = "tcp"
	TCP4 = "tcp4"
	TCP6 = "tcp6"
	UDS  = "unixgram"
)

type Sender interface {
	SendMetrics(ctx context.Context, metrics []byte) error
}

type StatsdSender struct {
	url      string
	protocol string
}

func New(url, protocol string) (*StatsdSender, error) {

	return &StatsdSender{
		url:      url,
		protocol: protocol,
	}, nil
}

func (s *StatsdSender) SendMetrics(ctx context.Context, metrics []byte) error {
	log.Printf("Sending metrics to %s via %s\n", s.url, s.protocol)

	switch s.protocol {
	case UDP, UDP4, UDP6:
		return s.sendUDPData(ctx, metrics)
	case TCP, TCP4, TCP6:
		return s.sendTCPData(ctx, metrics)
	case UDS:
		return s.sendUnixData(ctx, metrics)
	}

	return fmt.Errorf("unsupported protocol %s", s.protocol)
}

func (s *StatsdSender) sendUDPData(ctx context.Context, metrics []byte) error {
	log.Printf("Using %s protocol\n", s.protocol)

	// Resolve the UDP address
	udpAddr, err := net.ResolveUDPAddr(s.protocol, s.url)
	if err != nil {
		return fmt.Errorf("Error resolving address: %v", err)
	}

	// Create a UDP connection
	conn, err := net.DialUDP(s.protocol, nil, udpAddr)
	if err != nil {
		return fmt.Errorf("Error creating UDP connection: %v", err)
	}

	defer conn.Close()

	// Set a write deadline
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

	// Send the data
	_, err = conn.Write(metrics)
	if err != nil {
		return fmt.Errorf("Error sending data: %v", err)
	}

	return nil
}

func (s *StatsdSender) sendTCPData(ctx context.Context, metrics []byte) error {
	// Resolve the TCP address
	tcpAddr, err := net.ResolveTCPAddr(s.protocol, s.url)
	if err != nil {
		return fmt.Errorf("Error resolving TCP address: %v", err)
	}

	// Create a TCP connection
	conn, err := net.DialTCP(s.protocol, nil, tcpAddr)
	if err != nil {
		return fmt.Errorf("Error creating TCP connection: %v", err)
	}

	defer conn.Close()

	// Set a write deadline
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

	// Send the data
	_, err = conn.Write(metrics)
	if err != nil {
		return fmt.Errorf("Error sending data: %v", err)
	}

	return nil
}

func (s *StatsdSender) sendUnixData(ctx context.Context, metrics []byte) error {
	// Resolve the Unixgram address
	addr, err := net.ResolveUnixAddr(s.protocol, s.url)
	if err != nil {
		return fmt.Errorf("Error resolving Unix address: %v", err)
	}

	// Create a Unixgram connection
	conn, err := net.DialUnix(s.protocol, nil, addr)
	if err != nil {
		return fmt.Errorf("Error creating Unixgram connection: %v", err)
	}
	defer conn.Close()

	// Send the data
	_, err = conn.Write(metrics)
	if err != nil {
		return fmt.Errorf("Error sending data: %v", err)
	}

	return nil
}
