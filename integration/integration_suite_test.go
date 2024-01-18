package integration_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var blackboxPath string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	path, err := gexec.Build("code.cloudfoundry.org/blackbox/cmd/blackbox")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(gexec.CleanupBuildArtifacts)
	return []byte(path)
}, func(data []byte) {
	blackboxPath = string(data)
})

func Hostname() string {
	hostname, err := os.Hostname()
	Expect(err).NotTo(HaveOccurred())
	return hostname
}
