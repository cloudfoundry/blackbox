package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"

	"code.cloudfoundry.org/tlsconfig"

	"code.cloudfoundry.org/go-loggregator/v8/rfc5424"
)

type Drain struct {
	Transport string `yaml:"transport"`
	Address   string `yaml:"address"`
	CA        string `yaml:"ca"`
}

type Drainer interface {
	Drain(line string, tag string) error
}

const ServerPollingInterval = 5 * time.Second

type drainer struct {
	conn           net.Conn
	dialFunction   func() (net.Conn, error)
	errorLogger    *log.Logger
	hostname       string
	structuredData string
}

func NewDrainer(errorLogger *log.Logger, drain Drain, hostname, structuredData string) (*drainer, error) {
	var dialFunction func() (net.Conn, error)
	var tlsConf *tls.Config
	if len(drain.CA) != 0 {
		ca, err := ioutil.ReadFile(drain.CA)
		if err != nil {
			errorLogger.Printf("Error reading ca certificate: %s \n", err.Error())
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(ca)
		tlsConf, err = tlsconfig.Build(
			tlsconfig.WithExternalServiceDefaults(),
		).Client(
			tlsconfig.WithAuthority(certPool),
		)
		if err != nil {
			errorLogger.Printf("Error creating tls config: %s \n", err.Error())
			return nil, err
		}
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: time.Second * 60 * 3,
	}
	switch drain.Transport {
	case "tls":
		dialFunction = func() (net.Conn, error) {
			return tls.DialWithDialer(dialer, "tcp", drain.Address, tlsConf)
		}
	case "tcp":
		dialFunction = func() (net.Conn, error) {
			return dialer.Dial("tcp", drain.Address)
		}
	case "udp":
		dialFunction = func() (net.Conn, error) {
			return dialer.Dial("udp", drain.Address)
		}
	}

	return &drainer{
		hostname:       hostname,
		structuredData: structuredData,
		errorLogger:    errorLogger,
		dialFunction:   dialFunction,
	}, nil
}

func (d *drainer) Drain(line string, tag string) error {
	m := rfc5424.Message{
		Priority:  rfc5424.Info,
		Timestamp: time.Now(),
		Hostname:  d.hostname,
		AppName:   tag,
		ProcessID: "rs2",
		Message:   []byte(line),
		// StructuredData: d.structuredData,
	}

	binary, err := m.MarshalBinary()
	if err != nil {
		d.errorLogger.Printf("Error marshalling syslog: %s \n", err.Error())
		return err
	}
	for d.conn == nil {
		conn, err := d.dialFunction()
		if err != nil {
			d.errorLogger.Printf("Error connecting: %s \n", err.Error())
			time.Sleep(time.Second)
		}
		if conn != nil {
			d.conn = conn
		}
	}
	d.conn.SetWriteDeadline(time.Now().Add(time.Second * 30))
	_, err = d.conn.Write([]byte(strconv.Itoa(len(binary)) + " " + string(binary)))
	if err != nil {
		d.errorLogger.Printf("Error writing: %s \n", err.Error())
		d.conn = nil
	}
	return nil
}
