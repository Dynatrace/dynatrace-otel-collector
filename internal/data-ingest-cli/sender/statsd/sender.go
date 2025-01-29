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
	log.Printf("Sending metrics to %s via \n", s.url)

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
	// Resolve the UDP address
	udpAddr, err := net.ResolveUDPAddr(s.protocol, s.url)
	if err != nil {
		log.Fatalf("Error resolving address: %v", err)
		return err
	}

	// Create a UDP connection
	conn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		log.Fatalf("Error creating UDP connection: %v", err)
		return err
	}
	defer conn.Close()

	// Set a write deadline
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Send the data
	_, err = conn.Write(metrics)
	if err != nil {
		log.Fatalf("Error sending data: %v", err)
	}

	return err
}

func (s *StatsdSender) sendTCPData(ctx context.Context, metrics []byte) error {
	// Resolve the TCP address
	tcpAddr, err := net.ResolveTCPAddr(s.protocol, s.url)
	if err != nil {
		log.Fatalf("Error resolving TCP address: %v", err)
		return err
	}

	// Create a TCP connection
	conn, err := net.DialTCP("tcp4", nil, tcpAddr)
	if err != nil {
		log.Fatalf("Error creating TCP connection: %v", err)
		return err
	}
	defer conn.Close()

	// Set a write deadline
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

	// Send the data
	_, err = conn.Write(metrics)
	if err != nil {
		log.Fatalf("Error sending data: %v", err)
	}

	return err
}

func (s *StatsdSender) sendUnixData(ctx context.Context, metrics []byte) error {
	// Resolve the Unixgram address
	addr, err := net.ResolveUnixAddr(s.protocol, s.url)
	if err != nil {
		log.Fatalf("Error resolving Unix address: %v", err)
		return err
	}

	// Create a Unixgram connection
	conn, err := net.DialUnix(s.protocol, nil, addr)
	if err != nil {
		log.Fatalf("Error creating Unixgram connection: %v", err)
		return err
	}
	defer conn.Close()

	// Send the data
	_, err = conn.Write(metrics)
	if err != nil {
		log.Fatalf("Error sending data: %v", err)
	}

	return err
}
