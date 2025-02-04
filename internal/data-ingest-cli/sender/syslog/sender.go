// Copyright The OpenTelemetry Authors
// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package syslog

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config defines configuration for Syslog exporter.
type Config struct {
	// Syslog server address
	Endpoint string `mapstructure:"endpoint"`
	// Syslog server port
	Port int `mapstructure:"port"`
	// Transport for syslog communication
	// options: tcp, udp
	Transport string `mapstructure:"network"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.ClientConfig `mapstructure:"tls"`
}

type sender struct {
	transport string
	addr      string

	// Currently unset, but can be updated to test Syslog receivers with TLS enabled
	tlsConfig *tls.Config
	mu        sync.Mutex
	conn      net.Conn
}

func Connect(ctx context.Context, cfg *Config) (*sender, error) {
	s := &sender{
		transport: cfg.Transport,
		addr:      cfg.Endpoint,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.dial(ctx)
	if err != nil {
		return nil, err
	}
	return s, err
}

func (s *sender) dial(ctx context.Context) error {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	var err error
	if s.tlsConfig != nil && s.transport == string(confignet.TransportTypeTCP) {
		dialer := tls.Dialer{Config: s.tlsConfig}
		s.conn, err = dialer.DialContext(ctx, s.transport, s.addr)
	} else {
		dialer := new(net.Dialer)
		s.conn, err = dialer.DialContext(ctx, s.transport, s.addr)
	}
	return err
}

func (s *sender) Write(ctx context.Context, msgStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		if err := s.write(msgStr); err == nil {
			return nil
		}
	}
	if err := s.dial(ctx); err != nil {
		return err
	}

	return s.write(msgStr)
}

func (s *sender) write(msg string) error {
	// check if logs contains new line character at the end, if not add it
	if !strings.HasSuffix(msg, "\n") {
		msg = fmt.Sprintf("%s%s", msg, "\n")
	}
	_, err := fmt.Fprint(s.conn, msg)
	return err
}
