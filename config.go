package blackbox

import (
	"os"

	"code.cloudfoundry.org/blackbox/syslog"
	"gopkg.in/yaml.v3"
)

type SyslogConfig struct {
	Destination        syslog.Drain `yaml:"destination"`
	SourceDir          string       `yaml:"source_dir"`
	ExcludeFilePattern string       `yaml:"exclude_file_pattern"`
	LogFilename        bool         `yaml:"log_filename"`
}

type Config struct {
	Hostname          string            `yaml:"hostname"`
	StructuredDataID  string            `yaml:"structured_data_id"`
	StructuredDataMap map[string]string `yaml:"structured_data_map"`
	Syslog            SyslogConfig      `yaml:"syslog"`
	UseRFC3339        bool              `yaml:"use_rfc3339"`
	MaxMessageSize    int               `yaml:"max_message_size"`
}

func LoadConfig(path string) (*Config, error) {
	configFile, err := os.ReadFile(path)
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
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = 99990
	}
	if config.Syslog.Destination.Transport == "udp" {
		config.MaxMessageSize = 1024
	}

	return &config, nil
}
