package integration

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	. "github.com/onsi/ginkgo"

	"github.com/onsi/gomega/gbytes"
)

type TLSSyslogServer struct {
	Addr   string
	Buffer *gbytes.Buffer
	l      net.Listener
}

func (s *TLSSyslogServer) Run() error {
	// Listen for incoming connections.
	cer, err := tls.LoadX509KeyPair("./fixtures/server.crt", "./fixtures/server.key")
	if err != nil {
		log.Println(err)
		return err
	}

	config := &tls.Config{Certificates: []tls.Certificate{cer}}
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
				panic(err)
			}

		}
	}()

	return nil
}

func (s *TLSSyslogServer) Stop() {
	s.l.Close()
}
