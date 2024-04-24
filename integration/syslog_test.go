package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"

	. "code.cloudfoundry.org/blackbox/integration"

	sl "github.com/ziutek/syslog"

	"code.cloudfoundry.org/blackbox"
	"code.cloudfoundry.org/blackbox/syslog"
)

var _ = Describe("Blackbox", func() {
	var (
		logDir  string
		logFile *os.File
	)
	const (
		logfileName = "tail.log"
		tagName     = "test-tag"
	)

	BeforeEach(func() {
		var err error
		logDir, err = os.MkdirTemp("", "syslog-test")
		Expect(err).NotTo(HaveOccurred())

		err = os.Mkdir(filepath.Join(logDir, tagName), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		logFile, err = os.OpenFile(
			filepath.Join(logDir, tagName, logfileName),
			os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
			os.ModePerm,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		logFile.Close()

		err := os.RemoveAll(logDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when the syslog server is already running", func() {
		var (
			syslogServer   *SyslogServer
			blackboxRunner *BlackboxRunner
			inbox          *Inbox
		)

		BeforeEach(func() {
			inbox = NewInbox()
			syslogServer = NewSyslogServer(inbox)
			syslogServer.Start()

			blackboxRunner = NewBlackboxRunner(blackboxPath)
		})

		buildConfig := func(dirToWatch string) blackbox.Config {
			return blackbox.Config{
				Syslog: blackbox.SyslogConfig{
					Destination: syslog.Drain{
						Transport: "udp",
						Address:   syslogServer.Addr,
					},
					SourceDir:          dirToWatch,
					ExcludeFilePattern: "*.[0-9].log",
				},
				MaxMessageSize: 99990,
			}
		}

		AfterEach(func() {
			syslogServer.Stop()
		})

		Context("When using RFC3339 logging format", func() {
			JustBeforeEach(func() {
				buildConfig = func(dirToWatch string) blackbox.Config {
					return blackbox.Config{
						UseRFC3339: true,
						Syslog: blackbox.SyslogConfig{
							Destination: syslog.Drain{
								Transport: "udp",
								Address:   syslogServer.Addr,
							},
							SourceDir:          dirToWatch,
							ExcludeFilePattern: "*.[0-9].log",
						},
					}
				}
			})

			It("logs with requested format", func() {
				config := buildConfig(logDir)
				blackboxRunner.StartWithConfig(config, 1)

				Write(logFile, "hello\n", true, true)

				var message *sl.Message
				Eventually(inbox.Messages, "5s").Should(Receive(&message))
				Expect(message.Content).To(ContainSubstring("hello"))
				Expect(message.Content).To(ContainSubstring(tagName))
				Expect(message.Content).To(ContainSubstring(Hostname()))
				Expect(message.Content).To(MatchRegexp(`.*\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d*[Z\+].*`))

				blackboxRunner.Stop()
			})
		})

		It("logs any new lines of a file in source directory to syslog with subdirectory name as tag", func() {
			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", false, false)
			Write(logFile, "world\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Eventually(inbox.Messages, "2s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("world"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})

		It("creates messages with the expected priority", func() {
			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Facility).To(Equal(sl.User))
			Expect(message.Severity).To(Equal(sl.Info))
		})

		It("creates messages with the expected procid", func() {
			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(strings.Fields(message.Content)).To(ContainElement("rs2"))
		})

		It("truncates messages that are larger then configured limit", func() {
			address := fmt.Sprintf("127.0.0.1:%d", 9090+GinkgoParallelProcess())

			buffer := gbytes.NewBuffer()
			serverProcess := ginkgomon.Invoke(&TcpSyslogServer{
				Addr:   address,
				Buffer: buffer,
			})
			defer ginkgomon.Kill(serverProcess)

			config := buildConfig(logDir)
			config.MaxMessageSize = 2000
			config.Syslog.Destination.Transport = "tcp"
			config.Syslog.Destination.Address = address
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, strings.Repeat("a", 10000)+"\n", true, true)

			Eventually(buffer, "5s").Should(gbytes.Say("2000"))
			Eventually(len(buffer.Contents()), "5s").Should(Equal(2005))

			blackboxRunner.Stop()
		})

		It("truncates messages longer then 1024 in udp", func() {
			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, strings.Repeat("a", 10000)+"\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(len(message.Content)).To(Equal(1019))

			blackboxRunner.Stop()
		})

		Context("when logging with filename is activated", func() {
			It("logs with <subdirectory>/<filename> as tag", func() {
				config := buildConfig(logDir)
				config.Syslog.LogFilename = true
				blackboxRunner.StartWithConfig(config, 1)

				Write(logFile, "hello\n", true, true)

				var message *sl.Message
				Eventually(inbox.Messages, "5s").Should(Receive(&message))
				Expect(message.Content).To(ContainSubstring(tagName + "/" + logfileName))

				blackboxRunner.Stop()
			})
		})

		Context("tag name violates the constraints of the syslog message format", func() {
			It("cuts the tag name at 48 characters", func() {
				name50Chars := strings.Repeat("a", 50)
				expectedTagName48Chars := strings.Repeat("a", 48)

				err := os.Mkdir(filepath.Join(logDir, name50Chars), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
				logfile, err := os.OpenFile(
					filepath.Join(logDir, name50Chars, "example.log"),
					os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
					os.ModePerm,
				)
				Expect(err).NotTo(HaveOccurred())

				config := buildConfig(logDir)
				blackboxRunner.StartWithConfig(config, 1)

				var message *sl.Message

				Write(logfile, "hello \n", true, true)

				Eventually(inbox.Messages, "5s").Should(Receive(&message))
				Expect(message.Content).ToNot(ContainSubstring(name50Chars))
				Expect(message.Content).To(ContainSubstring(" " + expectedTagName48Chars + " "))

				blackboxRunner.Stop()
			})
			It("removes all characters that are not between ASCII 33 - 126 from the tag name", func() {
				specialCharsName := "ab c§d "
				expectedNoSpecialCharsName := "abcd"

				err := os.Mkdir(filepath.Join(logDir, specialCharsName), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				logfile, err := os.OpenFile(
					filepath.Join(logDir, specialCharsName, "example.log"),
					os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
					os.ModePerm,
				)
				Expect(err).NotTo(HaveOccurred())

				config := buildConfig(logDir)
				blackboxRunner.StartWithConfig(config, 1)

				Write(logfile, "hello \n", true, true)

				var message *sl.Message
				Eventually(inbox.Messages, "5s").Should(Receive(&message))
				Expect(message.Content).ToNot(ContainSubstring(specialCharsName))
				Expect(message.Content).To(ContainSubstring(" " + expectedNoSpecialCharsName + " "))

				blackboxRunner.Stop()
			})
		})

		It("can have a custom hostname", func() {
			config := buildConfig(logDir)
			config.Hostname = "fake-hostname"
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring("fake-hostname"))

			blackboxRunner.Stop()
		})

		It("can have structured data", func() {
			config := buildConfig(logDir)
			config.StructuredDataID = "StructuredData@1"
			config.StructuredDataMap = map[string]string{"test": "1"}
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring("[StructuredData@1 test=\"1\"]"))

			blackboxRunner.Stop()
		})

		It("does not log existing messages", func() {
			Write(logFile, "already present\n", true, false)

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "2s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))

			blackboxRunner.Stop()
		})

		It("tracks logs in multiple files in subdirectories of source directory", func() {
			anotherLogFile, err := os.OpenFile(
				filepath.Join(logDir, tagName, "another-tail.log"),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())
			defer anotherLogFile.Close()

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 2)

			Write(logFile, "hello\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Write(anotherLogFile, "hello from the other side\n", true, false)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello from the other side"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})

		It("skips files not ending in .log", func() {
			anotherLogFile, err := os.OpenFile(
				filepath.Join(logDir, tagName, "another-tail.log"),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())
			defer anotherLogFile.Close()
			notALogFile, err := os.OpenFile(filepath.Join(logDir, tagName, "not-a-log-file.log.1"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			defer notALogFile.Close()

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 2)

			Write(logFile, "hello\n", true, false)

			Write(notALogFile, "john cena\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "30s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Consistently(inbox.Messages).ShouldNot(Receive())

			Write(anotherLogFile, "hello from the other side\n", true, false)

			Write(notALogFile, "my time is now\n", true, false)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello from the other side"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Consistently(inbox.Messages).ShouldNot(Receive())

			blackboxRunner.Stop()
		})

		It("skips files matching exclude_file_pattern", func() {
			anotherLogFile, err := os.OpenFile(
				filepath.Join(logDir, tagName, "another-tail.log"),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())
			defer anotherLogFile.Close()
			notALogFile, err := os.OpenFile(filepath.Join(logDir, tagName, "excluded.1.log"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			defer notALogFile.Close()

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 2)

			Write(logFile, "hello\n", true, false)

			Write(notALogFile, "john cena\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "30s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring("test-tag"))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Consistently(inbox.Messages).ShouldNot(Receive())

			Write(anotherLogFile, "hello from the other side\n", true, false)

			Write(notALogFile, "my time is now\n", true, false)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello from the other side"))
			Expect(message.Content).To(ContainSubstring("test-tag"))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Consistently(inbox.Messages).ShouldNot(Receive())

			blackboxRunner.Stop()
		})

		It("tracks files in multiple directories using multiple tags", func() {
			tagName2 := "2-test-2-tag"
			err := os.Mkdir(filepath.Join(logDir, tagName2), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			anotherLogFile, err := os.OpenFile(
				filepath.Join(logDir, tagName2, "another-tail.log"),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())
			defer anotherLogFile.Close()

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 2)

			Write(logFile, "hello\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Write(anotherLogFile, "hello from the other side\n", true, false)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello from the other side"))
			Expect(message.Content).To(ContainSubstring("2-test-2-tag"))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})

		It("starts tracking logs in newly created files", func() {
			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			anotherLogFile, err := os.OpenFile(
				filepath.Join(logDir, tagName, "another-tail.log"),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())
			defer anotherLogFile.Close()

			// wait for tailer to pick up file, twice the interval
			time.Sleep(10 * time.Second)

			Write(anotherLogFile, "hello from the other side\n", true, false)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello from the other side"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			By("keeping track of old files")
			Write(logFile, "hello\n", true, false)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})

		It("continues discovering new files after the original files get deleted", func() {
			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			err := os.Rename(filepath.Join(logDir, tagName, logfileName), filepath.Join(logDir, tagName, "tail.log.1"))
			Expect(err).NotTo(HaveOccurred())

			// wait for tail process to die, tailer interval is 1 sec
			time.Sleep(2 * time.Second)

			anotherLogFile, err := os.OpenFile(
				filepath.Join(logDir, tagName, logfileName),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())
			defer anotherLogFile.Close()

			// wait for tailer to pick up file, twice the interval
			time.Sleep(10 * time.Second)

			Write(anotherLogFile, "bye\n", true, false)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("bye"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})

		It("does not ignore subdirectories in tag directories", func() {
			err := os.Mkdir(filepath.Join(logDir, tagName, "do-not-ignore-me"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			childLog, err := os.OpenFile(
				filepath.Join(logDir, tagName, "do-not-ignore-me", "and-my-son.log"),
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			Write(childLog, "child data\n", true, true)

			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("child data"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})

		It("ignores files in source directory", func() {
			err := os.WriteFile(
				filepath.Join(logDir, "not-a-tag-dir.log"),
				[]byte("some-data"),
				os.ModePerm,
			)
			Expect(err).NotTo(HaveOccurred())

			config := buildConfig(logDir)
			blackboxRunner.StartWithConfig(config, 1)

			Write(logFile, "hello\n", true, true)

			var message *sl.Message
			Eventually(inbox.Messages, "5s").Should(Receive(&message))
			Expect(message.Content).To(ContainSubstring("hello"))
			Expect(message.Content).To(ContainSubstring(tagName))
			Expect(message.Content).To(ContainSubstring(Hostname()))

			blackboxRunner.Stop()
		})
	})

	Context("when the syslog server is not already running", func() {
		var serverProcess ifrit.Process

		AfterEach(func() {
			ginkgomon.Interrupt(serverProcess)
		})

		It("tails files when server takes a long time to start", func() {
			address := fmt.Sprintf("127.0.0.1:%d", 9090+GinkgoParallelProcess())

			config := blackbox.Config{
				Hostname: "",
				Syslog: blackbox.SyslogConfig{
					Destination: syslog.Drain{
						Transport: "tcp",
						Address:   address,
					},
					SourceDir: logDir,
				},
			}

			configPath := CreateConfigFile(config)

			blackboxCmd := exec.Command(blackboxPath, "-config", configPath)

			session, err := gexec.Start(blackboxCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(2 * time.Second)

			buffer := gbytes.NewBuffer()
			serverProcess = ginkgomon.Invoke(&TcpSyslogServer{
				Addr:   address,
				Buffer: buffer,
			})

			Eventually(session.Err, "10s").Should(gbytes.Say("Starting to tail file:"))

			Write(logFile, "hello\n", false, false)
			Write(logFile, "world\n", true, false)

			Eventually(buffer, "5s").Should(gbytes.Say("hello"))
			Eventually(buffer, "5s").Should(gbytes.Say("world"))

			ginkgomon.Interrupt(serverProcess)
			// TODO: Figure out if we can make sure this log goes through
			Write(logFile, "can't log this\n", false, false)
			Write(logFile, "more\n", true, true)

			Eventually(session.Err, "5s").Should(gbytes.Say("Error connecting.*Will retry in 1 seconds"))
			Eventually(session.Err, "5s").Should(gbytes.Say("Error connecting.*Will retry in 1 seconds"))

			time.Sleep(2 * time.Second)

			buffer2 := gbytes.NewBuffer()
			serverProcess = ginkgomon.Invoke(&TcpSyslogServer{
				Addr:   address,
				Buffer: buffer2,
			})

			Eventually(buffer2, "10s").Should(gbytes.Say("more"))

			ginkgomon.Interrupt(serverProcess)

			if runtime.GOOS == "windows" {
				session.Kill()
			} else {
				session.Signal(os.Interrupt)
				session.Wait()
			}
		})
	})

	Context("When the server uses tls", func() {
		var address string
		var buffer *gbytes.Buffer
		var tlsserver TLSSyslogServer
		var blackboxRunner *BlackboxRunner

		BeforeEach(func() {
			address = fmt.Sprintf("127.0.0.1:%d", 9090+GinkgoParallelProcess())
			buffer = gbytes.NewBuffer()
			tlsserver = TLSSyslogServer{
				Addr:   address,
				Buffer: buffer,
			}
			err := tlsserver.Run()
			Expect(err).ToNot(HaveOccurred())
			blackboxRunner = NewBlackboxRunner(blackboxPath)
		})

		AfterEach(func() {
			tlsserver.Stop()
		})

		It("can send messages using tls", func() {
			blackboxConfig := blackbox.Config{
				Hostname: "",
				Syslog: blackbox.SyslogConfig{
					Destination: syslog.Drain{
						Transport: "tls",
						Address:   address,
						CA:        "./fixtures/ca.crt",
					},
					SourceDir: logDir,
				},
			}
			blackboxRunner.StartWithConfig(blackboxConfig, 1)
			Write(logFile, "hello\n", true, true)

			Eventually(buffer, "5s").Should(gbytes.Say("hello"))

			blackboxRunner.Stop()
		})
	})

	Context("When the server uses bad tls", func() {
		var address string
		var buffer *gbytes.Buffer
		var tlsserver TLSSyslogServer
		var blackboxRunner *BlackboxRunner

		BeforeEach(func() {
			address = fmt.Sprintf("127.0.0.1:%d", 9090+GinkgoParallelProcess())
			buffer = gbytes.NewBuffer()
			tlsserver = TLSSyslogServer{
				Addr:               address,
				Buffer:             buffer,
				CertPrefixOverride: "./fixtures/server-bad",
			}
			err := tlsserver.Run()
			Expect(err).ToNot(HaveOccurred())
			blackboxRunner = NewBlackboxRunner(blackboxPath)
		})

		AfterEach(func() {
			tlsserver.Stop()
		})

		It("doesn't cause blackbox to fail", func() {
			blackboxConfig := blackbox.Config{
				Hostname: "",
				Syslog: blackbox.SyslogConfig{
					Destination: syslog.Drain{
						Transport: "tls",
						Address:   address,
						CA:        "./fixtures/ca.crt",
					},
					SourceDir: logDir,
				},
			}
			blackboxRunner.StartWithConfig(blackboxConfig, 1)
			Write(logFile, "hello\n", true, true)

			Consistently(blackboxRunner.ExitChannel(), 10).ShouldNot(Receive())

			blackboxRunner.Stop()
		})
	})

	Context("when max retries is configured", func() {
		Context("when the syslog server never comes up", func() {
			It("causes blackbox to fail", func() {
				address := fmt.Sprintf("127.0.0.1:%d", 9090+GinkgoParallelProcess())

				config := blackbox.Config{
					Hostname: "",
					Syslog: blackbox.SyslogConfig{
						Destination: syslog.Drain{
							Transport:  "tcp",
							Address:    address,
							MaxRetries: 3,
						},
						SourceDir: logDir,
					},
				}
				configPath := CreateConfigFile(config)
				defer os.Remove(configPath)

				blackboxCmd := exec.Command(blackboxPath, "-config", configPath)
				session, err := gexec.Start(blackboxCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					if runtime.GOOS == "windows" {
						session.Kill()
					} else {
						session.Signal(os.Interrupt)
						session.Wait()
					}
				}()
				Eventually(session.Err, "10s").Should(gbytes.Say("Starting to tail file:"))

				Write(logFile, "try to log this\n", false, false)
				Write(logFile, "try to log more and notice can't write to socket\n", true, true)

				Eventually(session.Err, "5s").Should(gbytes.Say("Error connecting.*Will retry in 2 seconds"))
				Eventually(session.Err, "5s").Should(gbytes.Say("Error connecting.*Will retry in 4 seconds"))

				Expect(session.Wait("20s")).To(gexec.Exit(1))
			})
		})

		Context("when the syslog server goes down for a long enough time", func() {
			It("causes blackbox to fail", func() {
				address := fmt.Sprintf("127.0.0.1:%d", 9090+GinkgoParallelProcess())

				buffer := gbytes.NewBuffer()
				serverProcess := ginkgomon.Invoke(&TcpSyslogServer{
					Addr:   address,
					Buffer: buffer,
				})

				config := blackbox.Config{
					Hostname: "",
					Syslog: blackbox.SyslogConfig{
						Destination: syslog.Drain{
							Transport:  "tcp",
							Address:    address,
							MaxRetries: 1,
						},
						SourceDir: logDir,
					},
				}
				configPath := CreateConfigFile(config)
				defer os.Remove(configPath)

				blackboxCmd := exec.Command(blackboxPath, "-config", configPath)
				session, err := gexec.Start(blackboxCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					if runtime.GOOS == "windows" {
						session.Kill()
					} else {
						session.Signal(os.Interrupt)
						session.Wait()
					}
				}()
				Eventually(session.Err, "10s").Should(gbytes.Say("Starting to tail file:"))

				Write(logFile, "hello\n", false, false)
				Write(logFile, "world\n", true, false)

				Eventually(buffer, "5s").Should(gbytes.Say("hello"))
				Eventually(buffer, "5s").Should(gbytes.Say("world"))

				ginkgomon.Interrupt(serverProcess)

				Write(logFile, "try to log this\n", false, false)
				Write(logFile, "try to log more and notice can't write to socket\n", true, true)

				Expect(session.Wait("10s")).To(gexec.Exit(1))
			})
		})
	})
})

func Write(file *os.File, line string, sync bool, close bool) {
	_, err := file.WriteString(line)
	Expect(err).ToNot(HaveOccurred())
	if sync {
		err = file.Sync()
		Expect(err).ToNot(HaveOccurred())
	}
	if close {
		err = file.Close()
		Expect(err).ToNot(HaveOccurred())
	}
}
