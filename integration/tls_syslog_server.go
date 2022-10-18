package integration

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	"github.com/onsi/gomega/gbytes"
)

type TLSSyslogServer struct {
	Addr               string
	CertPrefixOverride string
	Buffer             *gbytes.Buffer
	l                  net.Listener
}

func (s *TLSSyslogServer) Run() error {
	pool, err := x509.SystemCertPool()
	if pool == nil {
		pool = x509.NewCertPool()
	}
	ca_cert, err := os.ReadFile("./fixtures/ca.crt")
	if err != nil {
		return err
	}
	if ok := pool.AppendCertsFromPEM(ca_cert); !ok {
		return errors.New("failed to apped CA")
	}

	certPrefix := "fixtures/server"
	if s.CertPrefixOverride != "" {
		certPrefix = s.CertPrefixOverride
	}
	cer, err := tls.LoadX509KeyPair(certPrefix+".crt", certPrefix+".key")
	if err != nil {
		log.Println(err)
		return err
	}
	config := &tls.Config{RootCAs: pool, Certificates: []tls.Certificate{cer}}

	// Listen for incoming connections.
	s.l, err = tls.Listen("tcp", s.Addr, config)
	if err != nil {
		return err
	}

	// Close the listener when the application closes.
	fmt.Println("Listening on " + s.Addr)
	var conn net.Conn
	go func() {
		for {
			defer GinkgoRecover()
			// Listen for an incoming connection.
			conn, err = s.l.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}
			defer conn.Close()

			_, err = io.Copy(s.Buffer, conn)

			// io.Copy is blocking. So when we close the underlying connection after
			// being signalled, we need to check for that error
			if err != nil {
				newErr, ok := err.(*net.OpError)
				if ok {
					if strings.Contains(newErr.Error(), "use of closed network connection") {
						return
					}
				}
				fmt.Println(err)
			}

		}
	}()

	return nil
}

func (s *TLSSyslogServer) Stop() {
	s.l.Close()
}
