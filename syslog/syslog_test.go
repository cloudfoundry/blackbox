package syslog

// Copyright (c) 2013 Paul Morton, Papertrail, Inc., & Paul Hammond

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const clienthost = "clienthost"

func panicf(s string, i ...interface{}) { panic(fmt.Sprintf(s, i...)) }

type testServer struct {
	Addr     string
	Close    chan bool
	Messages chan string
}

func newTestServer(network string) *testServer {
	server := testServer{
		Close:    make(chan bool, 1),
		Messages: make(chan string, 20),
	}
	switch network {
	case "tcp":
		ln := server.listenTCP()
		go server.serveTCP(ln)
	case "udp":
		conn := server.listenUDP()
		go server.serveUDP(conn)
	}
	return &server
}

func (s *testServer) listenTCP() net.Listener {
	addr := s.Addr
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panicf("listen error %v", err)
	}
	if s.Addr == "" {
		s.Addr = ln.Addr().String()
	}
	return ln
}

func (s *testServer) serveTCP(ln net.Listener) {
	for i := 0; ; i++ {
		select {
		case <-s.Close:
			ln.Close()
			return
		default:
			conn, err := ln.Accept()
			if err != nil {
				panicf("Accept error: %v", err)
			}
			go handle(conn, s.Messages)
			if !testing.Short() && 0 == i%5 {
				ln.Close()
				time.Sleep(time.Second * 6)
				ln = s.listenTCP()
			}
		}
	}
}

func (s *testServer) listenUDP() *net.UDPConn {
	addr := s.Addr
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		panicf("unexpected error %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		panicf("listen error %v", err)
	}
	if s.Addr == "" {
		s.Addr = conn.LocalAddr().String()
	}
	return conn
}

func (s *testServer) serveUDP(conn *net.UDPConn) {
	for {
		handle(conn, s.Messages)
		conn = s.listenUDP()
	}
}

func handle(conn io.ReadCloser, messages chan string) {

	for i := 0; ; i++ {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			panicf("Read error")
		} else {
			messages <- string(buf[0:n])
		}
		if i%2 == 0 {
			conn.Close()
			return
		}
	}
}

func generatePackets() []Packet {
	packets := make([]Packet, 10)
	for i := range packets {
		t, _ := time.Parse(time.RFC3339, "2016-10-05T18:30:00Z07:00")
		packets[i] = Packet{
			Severity: SevInfo,
			Facility: LogLocal1,
			Time:     t.UTC(),
			Hostname: clienthost,
			Tag:      "test",
			Message:  fmt.Sprintf("message %d", i),
		}
	}
	return packets
}

func TestParse(t *testing.T) {
	packets := generatePackets()
	for _, p := range packets {
		line := p.Generate(0)
		parsed, err := Parse(line)
		if err != nil {
			t.Fatal(err)
		}

		assert.EqualValues(t, p, parsed)
	}
}

func TestSyslog(t *testing.T) {
	for _, network := range []string{"tcp", "udp"} {
		s := newTestServer(network)

		connectTimeout := time.Duration(30) * time.Second
		writeTimeout := connectTimeout
		logger, err := Dial(clienthost, network, s.Addr, nil, connectTimeout, writeTimeout, 99990)
		if err != nil {
			t.Errorf("unexpected dial error %v", err)
		}
		packets := generatePackets()
		for _, p := range packets {
			logger.writePacket(p)
			time.Sleep(100 * time.Millisecond)
		}
		s.Close <- true

		for _, p := range packets {
			expected := p.Generate(0)
			if network == "tcp" {
				expected = expected + "\n"
			}
			select {
			case got := <-s.Messages:
				if got != expected {
					t.Errorf("expected %s, got %s", expected, got)
				}
			default:
				t.Errorf("expected %s, got nothing", expected)
				break
			}
		}
		if l := len(s.Messages); l != 0 {
			t.Errorf("found %d extra messages", l)
		}
	}
}
