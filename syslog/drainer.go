package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
	structuredData rfc5424.StructuredData
	maxMessageSize int
	transport      string
}

func NewDrainer(errorLogger *log.Logger, drain Drain, hostname string, structuredData rfc5424.StructuredData, maxMessageSize int) (*drainer, error) {
	tlsConf, err := generateTLSConfig(drain.CA)
	if err != nil {
		errorLogger.Printf(err.Error())
		return nil, err
	}

	dialFunction := generateDialer(drain, tlsConf)

	return &drainer{
		hostname:       hostname,
		structuredData: structuredData,
		errorLogger:    errorLogger,
		maxMessageSize: maxMessageSize,
		dialFunction:   dialFunction,
		transport:      drain.Transport,
	}, nil
}

func generateDialer(drain Drain, tlsConf *tls.Config) func() (net.Conn, error) {
	var dialFunction func() (net.Conn, error)
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
	return dialFunction
}

func generateTLSConfig(caString string) (*tls.Config, error) {
	if len(caString) == 0 {
		return nil, nil
	}

	ca, err := ioutil.ReadFile(caString)
	if err != nil {
		err = fmt.Errorf("Error reading ca certificate: %s \n", err.Error())
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)
	tlsConf, err := tlsconfig.Build(
		tlsconfig.WithExternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthority(certPool),
	)
	if err != nil {
		err = fmt.Errorf("Error creating tls config: %s \n", err.Error())
		return nil, err
	}

	return tlsConf, nil
}

func (d *drainer) Drain(line string, tag string) error {
	binary, err := d.formatMessage(line, tag)
	if err != nil {
		return err
	}
	for {
		d.ensureConnection()
		d.conn.SetWriteDeadline(time.Now().Add(time.Second * 30))
		if d.transport == "udp" {
			_, err = d.conn.Write(binary)
		} else {
			_, err = d.conn.Write([]byte(strconv.Itoa(len(binary)) + " " + string(binary)))
		}
		if err == nil {
			return nil
		}
		d.errorLogger.Printf("Error writing: %s \n", err.Error())
		d.conn.Close()
		d.conn = nil
		time.Sleep(time.Second)
	}
}

func (d *drainer) formatMessage(line string, tag string) ([]byte, error) {
	var structuredDatas []rfc5424.StructuredData
	if d.structuredData.ID != "" {
		structuredDatas = append(structuredDatas, d.structuredData)
	}
	m := rfc5424.Message{
		Priority:       rfc5424.Info,
		Timestamp:      time.Now(),
		Hostname:       d.hostname,
		AppName:        tag,
		ProcessID:      "cloudfoundry-blackbox",
		Message:        []byte(line),
		StructuredData: structuredDatas,
	}

	binary, err := m.MarshalBinary()
	if err != nil {
		d.errorLogger.Printf("Error marshalling syslog: %s \n", err.Error())
		return nil, err
	}
	if len(binary) > d.maxMessageSize {
		binary = binary[:d.maxMessageSize]
	}
	return binary, nil
}

func (d *drainer) ensureConnection() {
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
}
