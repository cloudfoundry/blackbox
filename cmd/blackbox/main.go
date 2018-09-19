package main

import (
	"flag"
	"log"
	"os"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/cloudfoundry/blackbox"
)

var configPath = flag.String(
	"config",
	"",
	"path to the configuration file",
)

func main() {
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)

	if *configPath == "" {
		logger.Fatalln("-config must be specified")
	}

	config, err := blackbox.LoadConfig(*configPath)
	if err != nil {
		logger.Fatalf("could not load config file: %s\n", err)
	}

	group := grouper.NewDynamic(nil, 0, 0)
	running := ifrit.Invoke(sigmon.New(group))

	go func() {
		fileWatcher := blackbox.NewFileWatcher(logger, config.Syslog.SourceDir, config.Syslog.LogFilename, group.Client(), config.Syslog.Destination, config.Hostname, config.StructuredData, config.Syslog.ExcludeFilePattern)
		fileWatcher.Watch()
	}()

	err = <-running.Wait()
	if err != nil {
		logger.Fatalf("failed: %s", err)
	}
}
