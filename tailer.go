package blackbox

import (
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nxadm/tail"
	"github.com/nxadm/tail/watch"

	"code.cloudfoundry.org/blackbox/syslog"
)

type Tailer struct {
	Path    string
	Tag     string
	Drainer syslog.Drainer
	Logger  *log.Logger
}

func (tailer *Tailer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watch.POLL_DURATION = 1 * time.Second

	tailer.Logger.Printf("Starting to tail file: %s", tailer.Path)
	t, err := tail.TailFile(tailer.Path, tail.Config{
		Follow: true,
		ReOpen: true,
		Poll:   true,
		Location: &tail.SeekInfo{
			Offset: 0,
			Whence: io.SeekEnd,
		},
		Logger: tailer.Logger,
	})

	if err != nil {
		return err
	}
	defer t.Cleanup()

	close(ready)

	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				log.Println("lines flushed; exiting tailer")
				return nil
			}

			lineTextNoCr := strings.TrimRight(line.Text, "\r")
			err = tailer.Drainer.Drain(lineTextNoCr, tailer.Tag)
			if err != nil {
				log.Println(err.Error())
			}
		case <-signals:
			return t.Stop()
		}
	}
}
