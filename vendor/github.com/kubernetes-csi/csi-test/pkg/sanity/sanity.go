/*
Copyright 2017 Luis Pabón luis@portworx.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sanity

import (
	"sync"
	"testing"

	"github.com/kubernetes-csi/csi-test/utils"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	driverAddress string
	conn          *grpc.ClientConn
	lock          sync.Mutex
)

// Test will test the CSI driver at the specified address
func Test(t *testing.T, address string) {
	lock.Lock()
	defer lock.Unlock()

	driverAddress = address
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSI Driver Test Suite")
}

var _ = BeforeSuite(func() {
	var err error
	conn, err = utils.Connect(driverAddress)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	conn.Close()
})
