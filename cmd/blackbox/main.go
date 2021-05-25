package main

import (
	"flag"
	"io"
	"log"
	"os"
	"time"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"code.cloudfoundry.org/blackbox"
)

var configPath = flag.String(
	"config",
	"",
	"path to the configuration file",
)

type LogWriter struct {
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	str := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") + " " + string(bytes)
	return io.WriteString(os.Stderr, str)
}

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

	if config.UseRFC3339 {
		logger = log.New(new(LogWriter), "", 0)
		log.SetOutput(new(LogWriter))
		log.SetFlags(0)
		logger.SetOutput(new(LogWriter))
		logger.SetFlags(0)
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
