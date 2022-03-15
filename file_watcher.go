package blackbox

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"code.cloudfoundry.org/blackbox/syslog"
	"code.cloudfoundry.org/go-loggregator/v8/rfc5424"
	"github.com/tedsuo/ifrit/grouper"
)

const POLL_INTERVAL = 5 * time.Second

type fileWatcher struct {
	logger *log.Logger

	sourceDir          string
	logFilename        bool
	dynamicGroupClient grouper.DynamicClient
	hostname           string
	structuredData     rfc5424.StructuredData
	excludeFilePattern string

	drain syslog.Drain
}

func NewFileWatcher(
	logger *log.Logger,
	sourceDir string,
	logFilename bool,
	dynamicGroupClient grouper.DynamicClient,
	drain syslog.Drain,
	hostname string,
	structuredData rfc5424.StructuredData,
	excludeFilePattern string,
) *fileWatcher {
	return &fileWatcher{
		logger:             logger,
		sourceDir:          sourceDir,
		logFilename:        logFilename,
		dynamicGroupClient: dynamicGroupClient,
		drain:              drain,
		hostname:           hostname,
		structuredData:     structuredData,
		excludeFilePattern: excludeFilePattern,
	}
}

func (f *fileWatcher) Watch() {
	for {
		logDirs, err := ioutil.ReadDir(f.sourceDir)
		if err != nil {
			f.logger.Fatalf("could not list directories in source dir: %s\n", err)
		}

		for _, logDir := range logDirs {
			tag := logDir.Name()
			tagDirPath := filepath.Join(f.sourceDir, tag)

			fileInfo, err := os.Stat(tagDirPath)
			if err != nil {
				f.logger.Fatalf("failed to determine if path is directory: %s\n", err)
			}

			if !fileInfo.IsDir() {
				continue
			}

			f.findLogsToWatch(tag, tagDirPath, fileInfo)

		}

		time.Sleep(POLL_INTERVAL)
	}
}

func (f *fileWatcher) findLogsToWatch(tag string, filePath string, file os.FileInfo) {
	if !file.IsDir() {
		if strings.HasSuffix(file.Name(), ".log") {
			if matched, _ := filepath.Match(f.excludeFilePattern, file.Name()); matched {
				return
			}
			if _, found := f.dynamicGroupClient.Get(filePath); !found {
				f.dynamicGroupClient.Inserter() <- f.memberForFile(filePath)
			}
		}
		return
	}

	dirContents, err := ioutil.ReadDir(filePath)
	if err != nil {
		f.logger.Printf("skipping log dir '%s' (could not list files): %s\n", tag, err)
		return
	}

	for _, content := range dirContents {
		currentFilePath := filepath.Join(filePath, content.Name())
		f.findLogsToWatch(tag, currentFilePath, content)
	}
}

func (f *fileWatcher) memberForFile(logfilePath string) grouper.Member {
	drainer, err := syslog.NewDrainer(f.logger, f.drain, f.hostname, f.structuredData)
	if err != nil {
		f.logger.Fatalf("could not drain to syslog: %s\n", err)
	}

	tag := f.determineTag(logfilePath)
	tag = f.formatSyslogAppName(tag, logfilePath)

	tailer := &Tailer{
		Path:    logfilePath,
		Tag:     tag,
		Drainer: drainer,
		Logger:  f.logger,
	}

	return grouper.Member{Name: tailer.Path, Runner: tailer}
}

func (f *fileWatcher) determineTag(logfilePath string) string {
	var tag string
	var err error
	if f.logFilename {
		tag, err = filepath.Rel(f.sourceDir, logfilePath)
	} else {
		logfileDir := filepath.Dir(logfilePath)
		tag, err = filepath.Rel(f.sourceDir, logfileDir)
	}
	if err != nil {
		f.logger.Fatalf("could not compute tag from file path %s: %s\n", logfilePath, err)
	}
	return tag
}

func (f *fileWatcher) formatSyslogAppName(originalAppname string, logfilePath string) string {
	//only ASCII chars from 33 to 126 are allowed
	forbiddenCharacters := "[^!-~]+"

	reg, err := regexp.Compile(forbiddenCharacters)
	if err != nil {
		f.logger.Fatalf("could not create regexp for sanitizing app-name %s: %s\n", logfilePath, err)
	}
	appname := reg.ReplaceAllString(originalAppname, "")

	if len(originalAppname) != len(appname) {
		f.logger.Printf("App-name consisted of chars outside of ASCII 33 to 126. app-name : %s, path: %s", originalAppname, logfilePath)
	}

	if len(appname) > 48 {
		f.logger.Printf("App-name was too long. Trimmed it to 48 Characters according to syslog formating rules, app-name : %s, path: %s", originalAppname, logfilePath)
		appname = appname[0:48]
	}
	return appname
}
