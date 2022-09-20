package storage

import (
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	storageutils "k8s.io/kubernetes/test/e2e/storage/utils"
	admissionapi "k8s.io/pod-security-admission/api"
)

type provisioningTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitCustomProvisioningTestSuite returns provisioningTestSuite that implements TestSuite interface
// using custom test patterns
func InitCustomProvisioningTestSuite(patterns []storageframework.TestPattern) storageframework.TestSuite {
	return &provisioningTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name:         "provisioning",
			TestPatterns: patterns,
		},
	}
}

// InitProvisioningTestSuite returns provisioningTestSuite that implements TestSuite interface\
// using test suite default patterns
func InitProvisioningTestSuite() storageframework.TestSuite {
	patterns := []storageframework.TestPattern{
		storageframework.DefaultFsDynamicPV,
		storageframework.BlockVolModeDynamicPV,
		storageframework.NtfsDynamicPV,
	}
	return InitCustomProvisioningTestSuite(patterns)
}

func (p *provisioningTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return p.tsInfo
}

func (p *provisioningTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	// Check preconditions.
	if pattern.VolType != storageframework.DynamicPV {
		e2eskipper.Skipf("Suite %q does not support %v", p.tsInfo.Name, pattern.VolType)
	}
	dInfo := driver.GetDriverInfo()
	if pattern.VolMode == v1.PersistentVolumeBlock && !dInfo.Capabilities[storageframework.CapBlock] {
		e2eskipper.Skipf("Driver %s doesn't support %v -- skipping", dInfo.Name, pattern.VolMode)
	}
}

func (p *provisioningTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		config        *storageframework.PerTestConfig
		driverCleanup func()

		cs        clientset.Interface
		pvc       *v1.PersistentVolumeClaim
		sourcePVC *v1.PersistentVolumeClaim
		sc        *storagev1.StorageClass
	}
	var (
		dInfo   = driver.GetDriverInfo()
		dDriver storageframework.DynamicPVTestDriver
		l       local
	)

	// Beware that it also registers an AfterEach which renders f unusable. Any code using
	// f must run inside an It or Context callback.
	f := framework.NewFrameworkWithCustomTimeouts("provisioning", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	init := func() {
		l = local{}
		dDriver, _ = driver.(storageframework.DynamicPVTestDriver)
		// Now do the more expensive test initialization.
		l.config, l.driverCleanup = driver.PrepareTest(f)
		l.cs = l.config.Framework.ClientSet
		testVolumeSizeRange := p.GetTestSuiteInfo().SupportedSizeRange
		driverVolumeSizeRange := dDriver.GetDriverInfo().SupportedSizeRange
		claimSize, err := storageutils.GetSizeRangesIntersection(testVolumeSizeRange, driverVolumeSizeRange)
		framework.ExpectNoError(err, "determine intersection of test size range %+v and driver size range %+v", testVolumeSizeRange, driverVolumeSizeRange)

		l.sc = dDriver.GetDynamicProvisionStorageClass(l.config, pattern.FsType)
		if l.sc == nil {
			e2eskipper.Skipf("Driver %q does not define Dynamic Provision StorageClass - skipping", dInfo.Name)
		}
		l.pvc = e2epv.MakePersistentVolumeClaim(e2epv.PersistentVolumeClaimConfig{
			ClaimSize:        claimSize,
			StorageClassName: &(l.sc.Name),
			VolumeMode:       &pattern.VolMode,
		}, l.config.Framework.Namespace.Name)
		l.sourcePVC = e2epv.MakePersistentVolumeClaim(e2epv.PersistentVolumeClaimConfig{
			ClaimSize:        claimSize,
			StorageClassName: &(l.sc.Name),
			VolumeMode:       &pattern.VolMode,
		}, l.config.Framework.Namespace.Name)
		framework.Logf("In creating storage class object and pvc objects for driver - sc: %v, pvc: %v, src-pvc: %v", l.sc, l.pvc, l.sourcePVC)
	}

	cleanup := func() {
		err := storageutils.TryFunc(l.driverCleanup)
		l.driverCleanup = nil
		framework.ExpectNoError(err, "while cleaning up driver")

	}

	ginkgo.It("should test case 1", func() {
		init()
		cleanup()
	})
}
