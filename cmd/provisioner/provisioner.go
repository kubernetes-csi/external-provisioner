/*
Copyright 2016 The Kubernetes Authors.

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

package main

import (
	//"fmt"
	//"os"
	"flag"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/kubernetes-incubator/external-storage/lib/controller"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

var (
	provisioner = flag.String("provisioner", "k8s.io/default", "Name of the provisioner. The provisioner will only provision volumes for claims that request a StorageClass with a provisioner field set equal to this name.")
	master      = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig  = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
	execCommand = flag.String("execCommand", "./flex-debug.sh/flexprov/flexprov", "The FlEX command/shell.")
)

type flexProvisioner struct {
	client      kubernetes.Interface
	execCommand string
	identity    string
	config      *rest.Config
}

var provisionController *controller.ProvisionController

func init() {

	flag.Parse()
	flag.Set("logtostderr", "true")

	// get the KUBECONFIG from env if specified (useful for local/debug cluster)
	kubeconfigEnv := os.Getenv("KUBECONFIG")

	if kubeconfigEnv != "" {
		glog.Infof("Found KUBECONFIG environment variable set, using that..")
		kubeconfig = &kubeconfigEnv
	}

	glog.Infof("FLEX Provisioner %s specified", *provisioner)
	if execCommand == nil {
		glog.Fatalf("Invalid flags specified: must provide provisioner exec command.")
	}

	var config *rest.Config
	var err error

	if *master != "" || *kubeconfig != "" {
		glog.Infof("Either master or kubeconfig specified. building kube config from that..")
		config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		glog.Infof("Building kube configs for running in cluster...")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Error getting server version: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	flexProvisioner := NewFlexProvisioner(clientset, *execCommand, "something")
	provisionController = controller.NewProvisionController(
		clientset,
		*provisioner,
		flexProvisioner,
		serverVersion.GitVersion,
	)

}



func NewFlexProvisioner(client kubernetes.Interface, execCommand string, identity string) controller.Provisioner {
	return newFlexProvisionerInternal(client, execCommand, identity)
}

func newFlexProvisionerInternal(client kubernetes.Interface, execCommand string, identity string) *flexProvisioner {

	provisioner := &flexProvisioner{
		client:      client,
		execCommand: execCommand,
		identity:    identity,
	}

	return provisioner
}

func (p *flexProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	glog.Infof("Provisioner %s Provision(..) called..", *provisioner)
	return nil, nil
}

func (p *flexProvisioner) Delete(volume *v1.PersistentVolume) error {
	glog.Infof("Provisioner %s Delete(..) called..", *provisioner)
	return nil
}

var _ controller.Provisioner = &flexProvisioner{}

func main() {

	provisionController.Run(wait.NeverStop)

}
