package e2e

import (
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ereporters "k8s.io/kubernetes/test/e2e/reporters"
)

var progressReporter = &e2ereporters.ProgressReporter{}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	// Reference common test to make the import valid.
	//commontest.CurrentSuite = commontest.E2E
	progressReporter.SetStartMsg()
	return nil
}, func(data []byte) {

})

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
// This function is called on each Ginkgo node in parallel mode.
func RunE2ETests(t *testing.T) {
	// Log failure immediately in addition to recording the test failure.
	progressReporter = e2ereporters.NewProgressReporter(framework.TestContext.ProgressReportURL)
	gomega.RegisterFailHandler(framework.Fail)

	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins

	if framework.TestContext.ReportDir != "" {
		if err := os.MkdirAll(framework.TestContext.ReportDir, 0755); err != nil {
			klog.Errorf("failed creating report directory: %v", err)
		}
	}
	ginkgo.RunSpecs(t, "external-provisioner e2e test suite")
}
