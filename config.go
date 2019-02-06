package blackbox

import (
	"io/ioutil"
	"os"

	yaml "github.com/bosh-tools/bosh-ext-cli/src/gopkg.in/yaml.v2"
	"github.com/cloudfoundry/blackbox/syslog"
)

type SyslogConfig struct {
	Destination        syslog.Drain `yaml:"destination"`
	SourceDir          string       `yaml:"source_dir"`
	ExcludeFilePattern string       `yaml:"exclude_file_pattern"`
	LogFilename        bool         `yaml:"log_filename"`
}

type Config struct {
	Hostname       string       `yaml:"hostname"`
	StructuredData string       `yaml:"structured_data"`
	Syslog         SyslogConfig `yaml:"syslog"`
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
