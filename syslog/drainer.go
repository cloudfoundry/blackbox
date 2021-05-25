package syslog

import (
	"crypto/x509"
	"errors"
	"io/ioutil"
	"log"
	"time"
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
	errorLogger    *log.Logger
	logger         *Logger
	hostname       string
	structuredData string
}

func NewDrainer(errorLogger *log.Logger, drain Drain, hostname, structuredData string) (*drainer, error) {
	err := errors.New("non-nil")
	var logger *Logger
	var certPool *x509.CertPool
	if len(drain.CA) != 0 {
		ca, err := ioutil.ReadFile(drain.CA)
		if err != nil {
			errorLogger.Printf("Error reading ca certificate: %s \n", err.Error())
			return nil, err
		}
		certPool = x509.NewCertPool()
		certPool.AppendCertsFromPEM(ca)
	}

	for err != nil {
		logger, err = Dial(
			hostname,
			drain.Transport,
			drain.Address,
			certPool,
			30*time.Second,
			30*time.Second,
			99990,
		)

		if err != nil {
			errorLogger.Printf("Connection error: %s, Will retry in %d \n", err.Error(), ServerPollingInterval)
			time.Sleep(ServerPollingInterval)
		}
	}

	if err != nil {
		return nil, err
	}

	return &drainer{
		logger:         logger,
		hostname:       hostname,
		structuredData: structuredData,
		errorLogger:    errorLogger,
	}, nil
}

func (d *drainer) Drain(line string, tag string) error {
	d.logger.Packets <- Packet{
		Severity:       SevInfo,
		Facility:       LogUser,
		StructuredData: d.structuredData,
		Hostname:       d.hostname,
		Tag:            tag,
		Time:           time.Now(),
		Message:        line,
	}

	select {
	case err := <-d.logger.Errors:
		d.errorLogger.Printf("Error sending syslog packet: %s", err)
		return err
	default:
		return nil
	}
}
