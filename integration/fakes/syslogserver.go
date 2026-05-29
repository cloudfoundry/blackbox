package fakes

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/onsi/gomega/gbytes"
)

// Protocol represents the protocol used by the SyslogServer.
type Protocol string

// Names for common protocols.
const (
	ProtocolTCP Protocol = "tcp"
	ProtocolTLS Protocol = "tls"
	ProtocolUDP Protocol = "udp"
)

type SyslogServer struct {
	// Address to listen on.
	address string
	// Path to the CA certificate file, if protocol is TLS.
	caPath string
	// Path to the certificate file, if protocol is TLS.
	certPath string
	// Path to the key file, if protocol is TLS.
	keyPath string
	// Protocol to listen with.
	protocol Protocol

	// Buffer to store incoming messages.
	Buf *gbytes.Buffer

	// Network listener.
	lis net.Listener
	// Network connection.
	conn net.Conn

	// Channel to signal when the server is stopped.
	stopped chan struct{}

	// Logger for syslog server.
	log *slog.Logger
}

// NewTCPSyslogServer creates a new TCP syslog server.
func NewTCPSyslogServer(addr string) *SyslogServer {
	return &SyslogServer{
		address:  addr,
		protocol: ProtocolTCP,
		Buf:      gbytes.NewBuffer(),
		stopped:  make(chan struct{}),
		log:      slog.Default().With("service", "fake syslog server"),
	}
}

// NewTLSSyslogServer creates a new TLS syslog server.
func NewTLSSyslogServer(addr, caPath, certPath, keyPath string) *SyslogServer {
	return &SyslogServer{
		address:  addr,
		caPath:   caPath,
		certPath: certPath,
		keyPath:  keyPath,
		protocol: ProtocolTLS,
		Buf:      gbytes.NewBuffer(),
		stopped:  make(chan struct{}),
		log:      slog.Default().With("service", "fake syslog server"),
	}
}

// Start starts the syslog server. It attempts to start a network listener with
// the server's protocol, returning an error if it fails. If the network
// listener is successfully started, an asynchronous loop is started to accept
// connections and store them in the buffer.
// Stop is expected to be called after Start.
// TODO: don't start if already stopped
func (s *SyslogServer) Start() error {
	s.log.Info("starting server", "address", s.address, "protocol", s.protocol)

	switch s.protocol {
	case ProtocolTCP:
		err := s.startTCP()
		if err != nil {
			return err
		}
	case ProtocolTLS:
		err := s.startTLS()
		if err != nil {
			return err
		}
	case ProtocolUDP:
		return nil
	default:
		return fmt.Errorf("unsupported protocol: %s", s.protocol)
	}

	s.serve()

	return nil
}

func (s *SyslogServer) startTCP() error {
	l, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}

	s.lis = l

	return nil
}

func (s *SyslogServer) startTLS() error {
	ca, err := os.ReadFile(s.caPath)
	if err != nil {
		return err
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(ca); !ok {
		return fmt.Errorf("could not append CA cert to pool")
	}

	cert, err := tls.LoadX509KeyPair(s.certPath, s.keyPath)
	if err != nil {
		return err
	}

	l, err := tls.Listen("tcp", s.address, &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		return err
	}

	s.lis = l

	return nil
}

// Serve starts a new goroutine that listens for incoming connections.
// Only one connection is accepted at a time.
func (s *SyslogServer) serve() {
	go func() {
		for {
			conn, err := s.lis.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					s.log.Error("accepting connection", "error", err)
				}
				close(s.stopped)
				return
			}
			s.conn = conn
			s.handleConnection(conn)
		}
	}()
}

func (s *SyslogServer) handleConnection(conn net.Conn) {
	s.log.Info("handling a connection")

	defer func() {
		_ = conn.Close()
	}()

	_, err := io.Copy(s.Buf, conn)
	if err != nil && !errors.Is(err, net.ErrClosed) {
		s.log.Error("copying from connection", "error", err)
	}
}

func (s *SyslogServer) Stop() error {
	s.log.Info("stopping server", "address", s.address, "protocol", s.protocol)

	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.lis != nil {
		_ = s.lis.Close()
	}

	<-s.stopped

	return nil
}
