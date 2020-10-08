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
	"context"
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	utilflag "k8s.io/component-base/cli/flag"
	csitrans "k8s.io/csi-translation-lib"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v6/controller"

	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/external-provisioner/pkg/capacity"
	"github.com/kubernetes-csi/external-provisioner/pkg/capacity/topology"
	ctrl "github.com/kubernetes-csi/external-provisioner/pkg/controller"
	"github.com/kubernetes-csi/external-provisioner/pkg/owner"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/client/v2/clientset/versioned"
)

var (
	master               = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig           = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
	csiEndpoint          = flag.String("csi-address", "/run/csi/socket", "The gRPC endpoint for Target CSI Volume.")
	volumeNamePrefix     = flag.String("volume-name-prefix", "pvc", "Prefix to apply to the name of a created volume.")
	volumeNameUUIDLength = flag.Int("volume-name-uuid-length", -1, "Truncates generated UUID of a created volume to this length. Defaults behavior is to NOT truncate.")
	showVersion          = flag.Bool("version", false, "Show version.")
	retryIntervalStart   = flag.Duration("retry-interval-start", time.Second, "Initial retry interval of failed provisioning or deletion. It doubles with each failure, up to retry-interval-max.")
	retryIntervalMax     = flag.Duration("retry-interval-max", 5*time.Minute, "Maximum retry interval of failed provisioning or deletion.")
	workerThreads        = flag.Uint("worker-threads", 100, "Number of provisioner worker threads, in other words nr. of simultaneous CSI calls.")
	finalizerThreads     = flag.Uint("cloning-protection-threads", 1, "Number of simultaneously running threads, handling cloning finalizer removal")
	capacityThreads      = flag.Uint("capacity-threads", 1, "Number of simultaneously running threads, handling CSIStorageCapacity objects")
	operationTimeout     = flag.Duration("timeout", 10*time.Second, "Timeout for waiting for creation or deletion of a volume")

	enableLeaderElection = flag.Bool("leader-election", false, "Enables leader election. If leader election is enabled, additional RBAC rules are required. Please refer to the Kubernetes CSI documentation for instructions on setting up these RBAC rules.")

	leaderElectionNamespace = flag.String("leader-election-namespace", "", "Namespace where the leader election resource lives. Defaults to the pod namespace if not set.")
	strictTopology          = flag.Bool("strict-topology", false, "Passes only selected node topology to CreateVolume Request, unlike default behavior of passing aggregated cluster topologies that match with topology keys of the selected node.")
	extraCreateMetadata     = flag.Bool("extra-create-metadata", false, "If set, add pv/pvc metadata to plugin create requests as parameters.")

	metricsAddress = flag.String("metrics-address", "", "The TCP network address where the prometheus metrics endpoint will listen (example: `:8080`). The default is empty string, which means metrics endpoint is disabled.")
	metricsPath    = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")

	defaultFSType = flag.String("default-fstype", "", "The default filesystem type of the volume to provision when fstype is unspecified in the StorageClass. If the default is not set and fstype is unset in the StorageClass, then no fstype will be set")

	kubeAPIQPS   = flag.Float32("kube-api-qps", 5, "QPS to use while communicating with the kubernetes apiserver. Defaults to 5.0.")
	kubeAPIBurst = flag.Int("kube-api-burst", 10, "Burst to use while communicating with the kubernetes apiserver. Defaults to 10.")

	capacityMode = func() *capacity.DeploymentMode {
		mode := capacity.DeploymentModeUnset
		flag.Var(&mode, "capacity-controller-deployment-mode", "Setting this enables producing CSIStorageCapacity objects with capacity information from the driver's GetCapacity call. 'central' is currently the only supported mode. Use it when there is just one active provisioner in the cluster.")
		return &mode
	}()
	capacityImmediateBinding = flag.Bool("capacity-for-immediate-binding", false, "Enables producing capacity information for storage classes with immediate binding. Not needed for the Kubernetes scheduler, maybe useful for other consumers or for debugging.")
	capacityPollInterval     = flag.Duration("capacity-poll-interval", time.Minute, "How long the external-provisioner waits before checking for storage capacity changes.")
	capacityOwnerrefLevel    = flag.Int("capacity-ownerref-level", 1, "The level indicates the number of objects that need to be traversed starting from the pod identified by the POD_NAME and POD_NAMESPACE environment variables to reach the owning object for CSIStorageCapacity objects: 0 for the pod itself, 1 for a StatefulSet, 2 for a Deployment, etc.")

	featureGates        map[string]bool
	provisionController *controller.ProvisionController
	version             = "unknown"
)

type leaderElection interface {
	Run() error
	WithNamespace(namespace string)
}

func main() {
	var config *rest.Config
	var err error

	flag.Var(utilflag.NewMapStringBool(&featureGates), "feature-gates", "A set of key=value pairs that describe feature gates for alpha/experimental features. "+
		"Options are:\n"+strings.Join(utilfeature.DefaultFeatureGate.KnownFeatures(), "\n"))

	klog.InitFlags(nil)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Set("logtostderr", "true")
	flag.Parse()

	if err := utilfeature.DefaultMutableFeatureGate.SetFromMap(featureGates); err != nil {
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

	config.QPS = *kubeAPIQPS
	config.Burst = *kubeAPIBurst

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	// snapclientset.NewForConfig creates a new Clientset for VolumesnapshotV1beta1Client
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

	metricsManager := metrics.NewCSIMetricsManager("" /* driverName */)

	grpcClient, err := ctrl.Connect(*csiEndpoint, metricsManager)
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
	metricsManager.SetDriverName(provisionerName)
	metricsManager.StartMetricsEndpoint(*metricsAddress, *metricsPath)

	pluginCapabilities, controllerCapabilities, err := ctrl.GetDriverCapabilities(grpcClient, *operationTimeout)
	if err != nil {
		klog.Fatalf("Error getting CSI driver capabilities: %s", err)
	}

	// Generate a unique ID for this provisioner
	timeStamp := time.Now().UnixNano() / int64(time.Millisecond)
	identity := strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + provisionerName

	factory := informers.NewSharedInformerFactory(clientset, ctrl.ResyncPeriodOfCsiNodeInformer)
	var factoryForNamespace informers.SharedInformerFactory // usually nil, only used for CSIStorageCapacity

	// -------------------------------
	// Listers
	// Create informer to prevent hit the API server for all resource request
	scLister := factory.Storage().V1().StorageClasses().Lister()
	claimLister := factory.Core().V1().PersistentVolumeClaims().Lister()

	var csiNodeLister storagelistersv1.CSINodeLister
	var vaLister storagelistersv1.VolumeAttachmentLister
	if controllerCapabilities[csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME] {
		klog.Info("CSI driver supports PUBLISH_UNPUBLISH_VOLUME, watching VolumeAttachments")
		vaLister = factory.Storage().V1().VolumeAttachments().Lister()
	} else {
		klog.Info("CSI driver does not support PUBLISH_UNPUBLISH_VOLUME, not watching VolumeAttachments")
	}
	var nodeLister v1.NodeLister
	if ctrl.SupportsTopology(pluginCapabilities) {
		csiNodeLister = factory.Storage().V1().CSINodes().Lister()
		nodeLister = factory.Core().V1().Nodes().Lister()
	}

	// -------------------------------
	// PersistentVolumeClaims informer
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(*retryIntervalStart, *retryIntervalMax)
	claimQueue := workqueue.NewNamedRateLimitingQueue(rateLimiter, "claims")
	claimInformer := factory.Core().V1().PersistentVolumeClaims().Informer()

	// Setup options
	provisionerOptions := []func(*controller.ProvisionController) error{
		controller.LeaderElection(false), // Always disable leader election in provisioner lib. Leader election should be done here in the CSI provisioner level instead.
		controller.FailedProvisionThreshold(0),
		controller.FailedDeleteThreshold(0),
		controller.RateLimiter(rateLimiter),
		controller.Threadiness(int(*workerThreads)),
		controller.CreateProvisionedPVLimiter(workqueue.DefaultControllerRateLimiter()),
		controller.ClaimsInformer(claimInformer),
	}

	translator := csitrans.New()

	supportsMigrationFromInTreePluginName := ""
	if translator.IsMigratedCSIDriverByName(provisionerName) {
		supportsMigrationFromInTreePluginName, err = translator.GetInTreeNameFromCSIName(provisionerName)
		if err != nil {
			klog.Fatalf("Failed to get InTree plugin name for migrated CSI plugin %s: %v", provisionerName, err)
		}
		klog.V(2).Infof("Supports migration from in-tree plugin: %s", supportsMigrationFromInTreePluginName)
		provisionerOptions = append(provisionerOptions, controller.AdditionalProvisionerNames([]string{supportsMigrationFromInTreePluginName}))
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	csiProvisioner := ctrl.NewCSIProvisioner(
		clientset,
		*operationTimeout,
		identity,
		*volumeNamePrefix,
		*volumeNameUUIDLength,
		grpcClient,
		snapClient,
		provisionerName,
		pluginCapabilities,
		controllerCapabilities,
		supportsMigrationFromInTreePluginName,
		*strictTopology,
		translator,
		scLister,
		csiNodeLister,
		nodeLister,
		claimLister,
		vaLister,
		*extraCreateMetadata,
		*defaultFSType,
	)

	provisionController = controller.NewProvisionController(
		clientset,
		provisionerName,
		csiProvisioner,
		serverVersion.GitVersion,
		provisionerOptions...,
	)

	csiClaimController := ctrl.NewCloningProtectionController(
		clientset,
		claimLister,
		claimInformer,
		claimQueue,
		controllerCapabilities,
	)

	var capacityController *capacity.Controller
	if *capacityMode == capacity.DeploymentModeCentral {
		podName := os.Getenv("POD_NAME")
		namespace := os.Getenv("POD_NAMESPACE")
		if podName == "" || namespace == "" {
			klog.Fatalf("need POD_NAMESPACE/POD_NAME env variables, have only POD_NAMESPACE=%q and POD_NAME=%q", namespace, podName)
		}
		controller, err := owner.Lookup(config, namespace, podName,
			schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}, *capacityOwnerrefLevel)
		if err != nil {
			klog.Fatalf("look up owner(s) of pod: %v", err)
		}
		klog.Infof("using %s/%s %s as owner of CSIStorageCapacity objects", controller.APIVersion, controller.Kind, controller.Name)

		topologyInformer := topology.NewNodeTopology(
			provisionerName,
			clientset,
			factory.Core().V1().Nodes(),
			factory.Storage().V1().CSINodes(),
			workqueue.NewNamedRateLimitingQueue(rateLimiter, "csitopology"),
		)

		// We only need objects from our own namespace. The normal factory would give
		// us an informer for the entire cluster.
		factoryForNamespace = informers.NewSharedInformerFactoryWithOptions(clientset,
			ctrl.ResyncPeriodOfCsiNodeInformer,
			informers.WithNamespace(namespace),
		)

		capacityController = capacity.NewCentralCapacityController(
			csi.NewControllerClient(grpcClient),
			provisionerName,
			clientset,
			// TODO: metrics for the queue?!
			workqueue.NewNamedRateLimitingQueue(rateLimiter, "csistoragecapacity"),
			*controller,
			namespace,
			topologyInformer,
			factory.Storage().V1().StorageClasses(),
			factoryForNamespace.Storage().V1alpha1().CSIStorageCapacities(),
			*capacityPollInterval,
			*capacityImmediateBinding,
		)
	}

	run := func(ctx context.Context) {
		factory.Start(ctx.Done())
		if factoryForNamespace != nil {
			// Starting is enough, the capacity controller will
			// wait for sync.
			factoryForNamespace.Start(ctx.Done())
		}
		cacheSyncResult := factory.WaitForCacheSync(ctx.Done())
		for _, v := range cacheSyncResult {
			if !v {
				klog.Fatalf("Failed to sync Informers!")
			}
		}

		if capacityController != nil {
			go capacityController.Run(ctx, int(*capacityThreads))
		}
		if csiClaimController != nil {
			go csiClaimController.Run(ctx, int(*finalizerThreads))
		}
		provisionController.Run(ctx)
	}

	if !*enableLeaderElection {
		run(context.TODO())
	} else {
		// this lock name pattern is also copied from sigs.k8s.io/sig-storage-lib-external-provisioner/v6/controller
		// to preserve backwards compatibility
		lockName := strings.Replace(provisionerName, "/", "-", -1)

		// create a new clientset for leader election
		leClientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			klog.Fatalf("Failed to create leaderelection client: %v", err)
		}

		le := leaderelection.NewLeaderElection(leClientset, lockName, run)

		if *leaderElectionNamespace != "" {
			le.WithNamespace(*leaderElectionNamespace)
		}

		if err := le.Run(); err != nil {
			klog.Fatalf("failed to initialize leader election: %v", err)
		}
	}

}
