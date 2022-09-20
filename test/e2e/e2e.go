package e2e

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
	"testing"
)

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
// This function is called on each Ginkgo node in parallel mode.
func RunE2ETests(t *testing.T) {
	// Log failure immediately in addition to recording the test failure.
	gomega.RegisterFailHandler(framework.Fail)

	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins
	//	var r []ginkgo.Reporter
	//	if framework.TestContext.ReportDir != "" {
	//		r = append(r, reporters.NewJUnitReporter(path.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%v%02d.xml", framework.TestContext.ReportPrefix, config.GinkgoConfig.))))
	//	}
	ginkgo.RunSpecs(t, "E2E suite")
}
