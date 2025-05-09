package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/tlsconfig"

	"code.cloudfoundry.org/go-loggregator/v10/rfc5424"
)

type Drain struct {
	Transport  string `yaml:"transport"`
	Address    string `yaml:"address"`
	CA         string `yaml:"ca"`
	MaxRetries int    `yaml:"max_retries"`
}

type Drainer interface {
	Drain(line string, tag string) error
}

type drainer struct {
	conn           net.Conn
	dialFunction   func() (net.Conn, error)
	errorLogger    *log.Logger
	hostname       string
	structuredData rfc5424.StructuredData
	maxMessageSize int
	transport      string
	maxRetries     int
	connAttempts   int
	sleepSeconds   int
}

func NewDrainer(errorLogger *log.Logger, drain Drain, hostname string, structuredData rfc5424.StructuredData, maxMessageSize int) (*drainer, error) {
	tlsConf, err := generateTLSConfig(drain.CA)
	if err != nil {
		errorLogger.Println("Error generating TLS config: ", err)
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
		maxRetries:     drain.MaxRetries,
		sleepSeconds:   1,
	}, nil
}

func generateDialer(drain Drain, tlsConf *tls.Config) func() (net.Conn, error) {
	var dialFunction func() (net.Conn, error)
	dialer := &net.Dialer{
		Timeout:   time.Second * 30,
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

	ca, err := os.ReadFile(caString)
	if err != nil {
		return nil, fmt.Errorf("error reading CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)
	tlsConf, err := tlsconfig.Build(
		tlsconfig.WithExternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthority(certPool),
	)
	if err != nil {
		return nil, err
	}

	return tlsConf, nil
}

func (d *drainer) Drain(line string, tag string) error {
	defer d.resetAttempts()

	binary, err := d.formatMessage(line, tag)
	if err != nil {
		return err
	}
	for {
		d.ensureConnection()
		err = d.conn.SetWriteDeadline(time.Now().Add(time.Second * 30))
		if err != nil {
			return err
		}
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
		Priority:       rfc5424.User | rfc5424.Info,
		Timestamp:      time.Now(),
		UseUTC:         true,
		Hostname:       d.hostname,
		AppName:        tag,
		ProcessID:      "rs2",
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

func (d *drainer) resetAttempts() {
	d.connAttempts = 0
	d.sleepSeconds = 1
}

func (d *drainer) incrementAttempts() {
	d.connAttempts++
	if d.maxRetries > 0 && d.sleepSeconds < 60 {
		d.sleepSeconds = d.sleepSeconds << 1
	}
}

func (d *drainer) ensureConnection() {
	for d.conn == nil {
		d.incrementAttempts()
		conn, err := d.dialFunction()
		if err != nil {
			if d.maxRetries > 0 && d.connAttempts > d.maxRetries {
				d.errorLogger.Fatalln("Failed to connect to syslog server. Exiting now.")
			}
			d.errorLogger.Printf("Error connecting on attempt %d: %s. Will retry in %d seconds.\n", d.connAttempts, err.Error(), d.sleepSeconds)
			time.Sleep(time.Second * time.Duration(d.sleepSeconds))
		} else if conn != nil {
			d.conn = conn
		}
	}
}
