/*
Copyright 2017 The Kubernetes Authors.

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
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
	"k8s.io/klog"

	ctrl "github.com/kubernetes-csi/external-provisioner/pkg/controller"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	csiclientset "k8s.io/csi-api/pkg/client/clientset/versioned"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilflag "k8s.io/apiserver/pkg/util/flag"
)

var (
	master               = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig           = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
	csiEndpoint          = flag.String("csi-address", "/run/csi/socket", "The gRPC endpoint for Target CSI Volume.")
	connectionTimeout    = flag.Duration("connection-timeout", 10*time.Second, "Timeout for waiting for CSI driver socket.")
	volumeNamePrefix     = flag.String("volume-name-prefix", "pvc", "Prefix to apply to the name of a created volume.")
	volumeNameUUIDLength = flag.Int("volume-name-uuid-length", -1, "Truncates generated UUID of a created volume to this length. Defaults behavior is to NOT truncate.")
	showVersion          = flag.Bool("version", false, "Show version.")
	enableLeaderElection = flag.Bool("enable-leader-election", false, "Enables leader election. If leader election is enabled, additional RBAC rules are required. Please refer to the Kubernetes CSI documentation for instructions on setting up these RBAC rules.")
	featureGates         map[string]bool

	provisionController *controller.ProvisionController
	version             = "unknown"
)

func init() {
	var config *rest.Config
	var err error

	flag.Var(utilflag.NewMapStringBool(&featureGates), "feature-gates", "A set of key=value pairs that describe feature gates for alpha/experimental features. "+
		"Options are:\n"+strings.Join(utilfeature.DefaultFeatureGate.KnownFeatures(), "\n"))

	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
	flag.Set("logtostderr", "true")

	if err := utilfeature.DefaultFeatureGate.SetFromMap(featureGates); err != nil {
		klog.Fatal(err)
	}

	if *showVersion {
		fmt.Println(os.Args[0], version)
		os.Exit(0)
	}
	klog.Infof("Version: %s", version)

	// get the KUBECONFIG from env if specified (useful for local/debug cluster)
	kubeconfigEnv := os.Getenv("KUBECONFIG")

	if kubeconfigEnv != "" {
		klog.Infof("Found KUBECONFIG environment variable set, using that..")
		kubeconfig = &kubeconfigEnv
	}

	if *master != "" || *kubeconfig != "" {
		klog.Infof("Either master or kubeconfig specified. building kube config from that..")
		config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		klog.Infof("Building kube configs for running in cluster...")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		klog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}
	// snapclientset.NewForConfig creates a new Clientset for VolumesnapshotV1alpha1Client
	snapClient, err := snapclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create snapshot client: %v", err)
	}
	csiAPIClient, err := csiclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create CSI API client: %v", err)
	}

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		klog.Fatalf("Error getting server version: %v", err)
	}

	// Provisioner will stay in Init until driver opens csi socket, once it's done
	// controller will exit this loop and proceed normally.
	socketDown := true
	grpcClient := &grpc.ClientConn{}
	for socketDown {
		grpcClient, err = ctrl.Connect(*csiEndpoint, *connectionTimeout)
		if err == nil {
			socketDown = false
			continue
		}
		time.Sleep(10 * time.Second)
	}

	// Autodetect provisioner name
	provisionerName, err := ctrl.GetDriverName(grpcClient, *connectionTimeout)
	if err != nil {
		klog.Fatalf("Error getting CSI driver name: %s", err)
	}
	klog.V(2).Infof("Detected CSI driver %s", provisionerName)

	// Generate a unique ID for this provisioner
	timeStamp := time.Now().UnixNano() / int64(time.Millisecond)
	identity := strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + provisionerName

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	csiProvisioner := ctrl.NewCSIProvisioner(clientset, csiAPIClient, *csiEndpoint, *connectionTimeout, identity, *volumeNamePrefix, *volumeNameUUIDLength, grpcClient, snapClient, provisionerName)
	provisionController = controller.NewProvisionController(
		clientset,
		provisionerName,
		csiProvisioner,
		serverVersion.GitVersion,
		controller.LeaderElection(*enableLeaderElection),
	)
}

func main() {
	provisionController.Run(wait.NeverStop)
}
