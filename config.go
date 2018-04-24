package blackbox

import (
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/blackbox/syslog"
	"gopkg.in/yaml.v2"
)

type SyslogConfig struct {
	Destination syslog.Drain `yaml:"destination"`
	SourceDir   string       `yaml:"source_dir"`
}

type Config struct {
	Hostname string `yaml:"hostname"`

	Syslog SyslogConfig `yaml:"syslog"`
}

func LoadConfig(path string) (*Config, error) {
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config

	if err := yaml.Unmarshal(configFile, &config); err != nil {
		return nil, err
	}

	if config.Hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		config.Hostname = hostname
	}

	return &config, nil
}
