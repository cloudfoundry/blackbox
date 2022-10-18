package integration

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/ziutek/syslog"

	"code.cloudfoundry.org/blackbox"
)

type SyslogServer struct {
	Addr string

	server *syslog.Server
}

func NewSyslogServer(inbox *Inbox) *SyslogServer {
	server := syslog.NewServer()
	server.AddHandler(inbox)

	return &SyslogServer{
		server: server,
	}
}

func (s *SyslogServer) Start() {
	port := fmt.Sprintf("%d", 9090+GinkgoParallelNode())
	l, err := net.Listen("tcp", ":"+port)
	Expect(err).NotTo(HaveOccurred())
	l.Close()

	addr := fmt.Sprintf("127.0.0.1:%s", port)
	err = s.server.Listen(addr)
	Expect(err).NotTo(HaveOccurred())

	s.Addr = addr
}

func (s *SyslogServer) Stop() {
	s.server.Shutdown()
	Eventually(func() error {
		_, err := net.Dial("tcp", s.Addr)
		return err
	}).ShouldNot(BeNil())

	s.Addr = ""
}

type Inbox struct {
	Messages chan *syslog.Message
}

func NewInbox() *Inbox {
	return &Inbox{
		Messages: make(chan *syslog.Message),
	}
}

func (i *Inbox) Handle(m *syslog.Message) *syslog.Message {
	if m == nil {
		close(i.Messages)
		return nil
	}

	i.Messages <- m
	return nil
}

type BlackboxRunner struct {
	blackboxPath    string
	blackboxProcess ifrit.Process
}

func NewBlackboxRunner(blackboxPath string) *BlackboxRunner {
	return &BlackboxRunner{
		blackboxPath: blackboxPath,
	}
}

func (runner *BlackboxRunner) StartWithConfig(config blackbox.Config, tailerCount int) {
	configPath := CreateConfigFile(config)

	blackboxCmd := exec.Command(runner.blackboxPath, "-config", configPath)
	blackboxRunner := ginkgomon.New(
		ginkgomon.Config{
			Name:          "blackbox",
			Command:       blackboxCmd,
			AnsiColorCode: "90m",
			StartCheck:    "Start tail...",
			Cleanup: func() {
				os.Remove(configPath)
			},
		},
	)

	runner.blackboxProcess = ginkgomon.Invoke(blackboxRunner)
}

func (runner *BlackboxRunner) ExitChannel() <-chan error {
	return runner.blackboxProcess.Wait()
}

func (runner *BlackboxRunner) Stop() {
	ginkgomon.Kill(runner.blackboxProcess)
}

func CreateConfigFile(config blackbox.Config) string {
	configFile, err := os.CreateTemp("", "blackbox_config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()

	yamlToWrite, err := yaml.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	_, err = configFile.Write(yamlToWrite)
	Expect(err).NotTo(HaveOccurred())

	return configFile.Name()
}
