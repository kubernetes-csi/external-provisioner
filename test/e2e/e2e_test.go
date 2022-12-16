package e2e

import (
	"flag"
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"

	// test sources
	_ "github.com/kubernetes-csi/external-provisioner/test/e2e/storage"
)

func TestMain(m *testing.M) {
	klog.InitFlags(nil)
	utilruntime.Must(flag.Set("logtostderr", "false"))
	utilruntime.Must(flag.Set("alsologtostderr", "false"))
	utilruntime.Must(flag.Set("one_output", "true"))
	klog.SetOutput(ginkgo.GinkgoWriter)

	// Register framework flags, then handle flags.
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()
	framework.AfterReadingAllFlags(&framework.TestContext)
	// Now run the test suite.
	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
