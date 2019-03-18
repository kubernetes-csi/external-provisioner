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

	"github.com/kubernetes-csi/csi-lib-utils/deprecatedflags"
	ctrl "github.com/kubernetes-csi/external-provisioner/pkg/controller"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/controller"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	csitranslationlib "k8s.io/csi-translation-lib"
)

var (
	master               = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig           = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
	csiEndpoint          = flag.String("csi-address", "/run/csi/socket", "The gRPC endpoint for Target CSI Volume.")
	_                    = deprecatedflags.Add("connection-timeout")
	volumeNamePrefix     = flag.String("volume-name-prefix", "pvc", "Prefix to apply to the name of a created volume.")
	volumeNameUUIDLength = flag.Int("volume-name-uuid-length", -1, "Truncates generated UUID of a created volume to this length. Defaults behavior is to NOT truncate.")
	showVersion          = flag.Bool("version", false, "Show version.")
	enableLeaderElection = flag.Bool("enable-leader-election", false, "Enables leader election. If leader election is enabled, additional RBAC rules are required. Please refer to the Kubernetes CSI documentation for instructions on setting up these RBAC rules.")
	retryIntervalStart   = flag.Duration("retry-interval-start", time.Second, "Initial retry interval of failed provisioning or deletion. It doubles with each failure, up to retry-interval-max.")
	retryIntervalMax     = flag.Duration("retry-interval-max", 5*time.Minute, "Maximum retry interval of failed provisioning or deletion.")
	workerThreads        = flag.Uint("worker-threads", 100, "Number of provisioner worker threads, in other words nr. of simultaneous CSI calls.")
	operationTimeout     = flag.Duration("timeout", 10*time.Second, "Timeout for waiting for creation or deletion of a volume")
	_                    = deprecatedflags.Add("provisioner")

	featureGates        map[string]bool
	provisionController *controller.ProvisionController
	version             = "unknown"
)

func init() {
	var config *rest.Config
	var err error

	flag.Var(utilflag.NewMapStringBool(&featureGates), "feature-gates", "A set of key=value pairs that describe feature gates for alpha/experimental features. "+
		"Options are:\n"+strings.Join(utilfeature.DefaultFeatureGate.KnownFeatures(), "\n"))

	klog.InitFlags(nil)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Set("logtostderr", "true")
	flag.Parse()

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

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		klog.Fatalf("Error getting server version: %v", err)
	}

	grpcClient, err := ctrl.Connect(*csiEndpoint)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	err = ctrl.Probe(grpcClient, *operationTimeout)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	// Autodetect provisioner name
	provisionerName, err := ctrl.GetDriverName(grpcClient, *operationTimeout)
	if err != nil {
		klog.Fatalf("Error getting CSI driver name: %s", err)
	}
	klog.V(2).Infof("Detected CSI driver %s", provisionerName)

	pluginCapabilities, controllerCapabilities, err := ctrl.GetDriverCapabilities(grpcClient, *operationTimeout)
	if err != nil {
		klog.Fatalf("Error getting CSI driver capabilities: %s", err)
	}

	// Generate a unique ID for this provisioner
	timeStamp := time.Now().UnixNano() / int64(time.Millisecond)
	identity := strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + provisionerName

	provisionerOptions := []func(*controller.ProvisionController) error{
		controller.LeaderElection(*enableLeaderElection),
		controller.FailedProvisionThreshold(0),
		controller.FailedDeleteThreshold(0),
		controller.RateLimiter(workqueue.NewItemExponentialFailureRateLimiter(*retryIntervalStart, *retryIntervalMax)),
		controller.Threadiness(int(*workerThreads)),
		controller.CreateProvisionedPVLimiter(workqueue.DefaultControllerRateLimiter()),
	}

	supportsMigrationFromInTreePluginName := ""
	if csitranslationlib.IsMigratedCSIDriverByName(provisionerName) {
		supportsMigrationFromInTreePluginName, err = csitranslationlib.GetInTreeNameFromCSIName(provisionerName)
		if err != nil {
			klog.Fatalf("Failed to get InTree plugin name for migrated CSI plugin %s: %v", provisionerName, err)
		}
		klog.V(2).Infof("Supports migration from in-tree plugin: %s", supportsMigrationFromInTreePluginName)
		provisionerOptions = append(provisionerOptions, controller.AdditionalProvisionerNames([]string{supportsMigrationFromInTreePluginName}))
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	csiProvisioner := ctrl.NewCSIProvisioner(clientset, *operationTimeout, identity, *volumeNamePrefix, *volumeNameUUIDLength, grpcClient, snapClient, provisionerName, pluginCapabilities, controllerCapabilities, supportsMigrationFromInTreePluginName)
	provisionController = controller.NewProvisionController(
		clientset,
		provisionerName,
		csiProvisioner,
		serverVersion.GitVersion,
		provisionerOptions...,
	)
}

func main() {
	provisionController.Run(wait.NeverStop)
}
