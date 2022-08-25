package storage

import (
	"context"
	"github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = ginkgo.Describe("Test1", func() {
	f := framework.NewDefaultFramework("pv")
	ginkgo.Context("test1", func() {
		ginkgo.It("test-1-A", func() {
			framework.Logf("Raunak was here")
			ns, err := f.ClientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
			framework.ExpectNoError(err, "get namespaces")
			framework.Logf("Namespaces: %v", ns)
		})
	})
})
