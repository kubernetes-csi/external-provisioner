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
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	server "k8s.io/apiserver/pkg/server"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/component-base/metrics/legacyregistry"
	_ "k8s.io/component-base/metrics/prometheus/clientgo/leaderelection" // register leader election in the default legacy registry
	_ "k8s.io/component-base/metrics/prometheus/workqueue"               // register work queues in the default legacy registry
	csitrans "k8s.io/csi-translation-lib"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v12/controller"
	libmetrics "sigs.k8s.io/sig-storage-lib-external-provisioner/v12/controller/metrics"

	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/csi-lib-utils/standardflags"
	"github.com/kubernetes-csi/external-provisioner/v5/pkg/capacity"
	"github.com/kubernetes-csi/external-provisioner/v5/pkg/capacity/topology"
	ctrl "github.com/kubernetes-csi/external-provisioner/v5/pkg/controller"
	"github.com/kubernetes-csi/external-provisioner/v5/pkg/features"
	"github.com/kubernetes-csi/external-provisioner/v5/pkg/owner"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gatewayInformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	referenceGrantv1beta1 "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1beta1"
)

var (
	volumeNamePrefix     = flag.String("volume-name-prefix", "pvc", "Prefix to apply to the name of a created volume.")
	volumeNameUUIDLength = flag.Int("volume-name-uuid-length", -1, "Truncates generated UUID of a created volume to this length. Defaults behavior is to NOT truncate.")
	workerThreads        = flag.Uint("worker-threads", 100, "Number of provisioner worker threads, in other words nr. of simultaneous CSI calls.")
	finalizerThreads     = flag.Uint("cloning-protection-threads", 1, "Number of simultaneously running threads, handling cloning finalizer removal")
	capacityThreads      = flag.Uint("capacity-threads", 1, "Number of simultaneously running threads, handling CSIStorageCapacity objects")
	operationTimeout     = flag.Duration("timeout", 10*time.Second, "Timeout for waiting for volume operation (creation, deletion, capacity queries)")

	strictTopology      = flag.Bool("strict-topology", false, "Late binding: pass only selected node topology to CreateVolume Request, unlike default behavior of passing aggregated cluster topologies that match with topology keys of the selected node.")
	immediateTopology   = flag.Bool("immediate-topology", true, "Immediate binding: pass aggregated cluster topologies for all nodes where the CSI driver is available (enabled, the default) or no topology requirements (if disabled).")
	extraCreateMetadata = flag.Bool("extra-create-metadata", false, "If set, add pv/pvc metadata to plugin create requests as parameters.")
	enableProfile       = flag.Bool("enable-pprof", false, "Enable pprof profiling on the TCP network address specified by --http-endpoint. The HTTP path is `/debug/pprof/`.")

	defaultFSType = flag.String("default-fstype", "", "The default filesystem type of the volume to provision when fstype is unspecified in the StorageClass. If the default is not set and fstype is unset in the StorageClass, then no fstype will be set")

	enableCapacity           = flag.Bool("enable-capacity", false, "This enables producing CSIStorageCapacity objects with capacity information from the driver's GetCapacity call.")
	capacityImmediateBinding = flag.Bool("capacity-for-immediate-binding", false, "Enables producing capacity information for storage classes with immediate binding. Not needed for the Kubernetes scheduler, maybe useful for other consumers or for debugging.")
	capacityPollInterval     = flag.Duration("capacity-poll-interval", time.Minute, "How long the external-provisioner waits before checking for storage capacity changes.")
	capacityOwnerrefLevel    = flag.Int("capacity-ownerref-level", 1, "The level indicates the number of objects that need to be traversed starting from the pod identified by the POD_NAME and NAMESPACE environment variables to reach the owning object for CSIStorageCapacity objects: -1 for no owner, 0 for the pod itself, 1 for a StatefulSet or DaemonSet, 2 for a Deployment, etc.")

	enableNodeDeployment           = flag.Bool("node-deployment", false, "Enables deploying the external-provisioner together with a CSI driver on nodes to manage node-local volumes.")
	nodeDeploymentImmediateBinding = flag.Bool("node-deployment-immediate-binding", true, "Determines whether immediate binding is supported when deployed on each node.")
	nodeDeploymentBaseDelay        = flag.Duration("node-deployment-base-delay", 20*time.Second, "Determines how long the external-provisioner sleeps initially before trying to own a PVC with immediate binding.")
	nodeDeploymentMaxDelay         = flag.Duration("node-deployment-max-delay", 60*time.Second, "Determines how long the external-provisioner sleeps at most before trying to own a PVC with immediate binding.")
	controllerPublishReadOnly      = flag.Bool("controller-publish-readonly", false, "This option enables PV to be marked as readonly at controller publish volume call if PVC accessmode has been set to ROX.")

	preventVolumeModeConversion = flag.Bool("prevent-volume-mode-conversion", true, "Prevents an unauthorised user from modifying the volume mode when creating a PVC from an existing VolumeSnapshot.")

	featureGates        map[string]bool
	provisionController *controller.ProvisionController
	version             = "unknown"
)

func main() {
	var config *rest.Config
	var err error

	flag.Var(utilflag.NewMapStringBool(&featureGates), "feature-gates", "A set of key=value pairs that describe feature gates for alpha/experimental features. "+
		"Options are:\n"+strings.Join(utilfeature.DefaultFeatureGate.KnownFeatures(), "\n"))

	fg := featuregate.NewFeatureGate()
	logsapi.AddFeatureGates(fg)
	c := logsapi.NewLoggingConfiguration()
	logsapi.AddFlags(c, flag.CommandLine)
	logs.InitLogs()
	standardflags.RegisterCommonFlags(goflag.CommandLine)
	standardflags.AddAutomaxprocs(klog.Infof)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
	if err := logsapi.ValidateAndApply(c, fg); err != nil {
		klog.ErrorS(err, "LoggingConfiguration is invalid")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	ctx := context.Background()

	if err := utilfeature.DefaultMutableFeatureGate.SetFromMap(featureGates); err != nil {
		klog.Fatal(err)
	}

	node := os.Getenv("NODE_NAME")
	if *enableNodeDeployment && node == "" {
		klog.Fatal("The NODE_NAME environment variable must be set when using --enable-node-deployment.")
	}

	if standardflags.Configuration.ShowVersion {
		fmt.Println(os.Args[0], version)
		os.Exit(0)
	}
	klog.InfoS("Version", "version", version)

	if standardflags.Configuration.MetricsAddress != "" && standardflags.Configuration.HttpEndpoint != "" {
		klog.Error("only one of `--metrics-address` and `--http-endpoint` can be set.")
		os.Exit(1)
	}
	addr := standardflags.Configuration.MetricsAddress
	if addr == "" {
		addr = standardflags.Configuration.HttpEndpoint
	}

	// get the KUBECONFIG from env if specified (useful for local/debug cluster)
	kubeconfigEnv := os.Getenv("KUBECONFIG")

	if kubeconfigEnv != "" {
		klog.Infof("Found KUBECONFIG environment variable set, using that..")
		standardflags.Configuration.KubeConfig = kubeconfigEnv
	}

	if standardflags.Configuration.Master != "" || standardflags.Configuration.KubeConfig != "" {
		klog.Infof("Either master or kubeconfig specified. building kube config from that..")
		config, err = clientcmd.BuildConfigFromFlags(standardflags.Configuration.Master, standardflags.Configuration.KubeConfig)
	} else {
		klog.Infof("Building kube configs for running in cluster...")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		klog.Fatalf("Failed to create config: %v", err)
	}

	config.QPS = float32(standardflags.Configuration.KubeAPIQPS)
	config.Burst = standardflags.Configuration.KubeAPIBurst

	coreConfig := rest.CopyConfig(config)
	coreConfig.ContentType = runtime.ContentTypeProtobuf
	clientset, err := kubernetes.NewForConfig(coreConfig)
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	// snapclientset.NewForConfig creates a new Clientset for  VolumesnapshotV1Client
	snapClient, err := snapclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create snapshot client: %v", err)
	}

	var gatewayClient gatewayclientset.Interface
	if utilfeature.DefaultFeatureGate.Enabled(features.CrossNamespaceVolumeDataSource) {
		// gatewayclientset.NewForConfig creates a new Clientset for GatewayClient
		gatewayClient, err = gatewayclientset.NewForConfig(config)
		if err != nil {
			klog.Fatalf("Failed to create gateway client: %v", err)
		}
	}

	metricsManager := metrics.NewCSIMetricsManagerWithOptions("", /* driverName */
		// Will be provided via default gatherer.
		metrics.WithProcessStartTime(false),
		metrics.WithSubsystem(metrics.SubsystemSidecar),
	)

	grpcClient, err := ctrl.Connect(ctx, standardflags.Configuration.CSIAddress, metricsManager)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}

	err = ctrl.Probe(ctx, grpcClient, *operationTimeout)
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

	translator := csitrans.New()
	supportsMigrationFromInTreePluginName := ""
	if translator.IsMigratedCSIDriverByName(provisionerName) {
		supportsMigrationFromInTreePluginName, err = translator.GetInTreeNameFromCSIName(provisionerName)
		if err != nil {
			klog.Fatalf("Failed to get InTree plugin name for migrated CSI plugin %s: %v", provisionerName, err)
		}
		klog.V(2).Infof("Supports migration from in-tree plugin: %s", supportsMigrationFromInTreePluginName)

		// Create a new connection with the metrics manager with migrated label
		metricsManager = metrics.NewCSIMetricsManagerWithOptions(provisionerName,
			// Will be provided via default gatherer.
			metrics.WithProcessStartTime(false),
			metrics.WithMigration())
		migratedGrpcClient, err := ctrl.Connect(ctx, standardflags.Configuration.CSIAddress, metricsManager)
		if err != nil {
			klog.Error(err.Error())
			os.Exit(1)
		}
		grpcClient.Close()
		grpcClient = migratedGrpcClient

		err = ctrl.Probe(ctx, grpcClient, *operationTimeout)
		if err != nil {
			klog.Error(err.Error())
			os.Exit(1)
		}
	}

	// Prepare http endpoint for metrics + leader election healthz
	mux := http.NewServeMux()
	gatherers := prometheus.Gatherers{
		// For workqueue and leader election metrics, set up via the anonymous imports of:
		// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/component-base/metrics/prometheus/workqueue/metrics.go
		// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/component-base/metrics/prometheus/clientgo/leaderelection/metrics.go
		//
		// Also to happens to include Go runtime and process metrics:
		// https://github.com/kubernetes/kubernetes/blob/9780d88cb6a4b5b067256ecb4abf56892093ee87/staging/src/k8s.io/component-base/metrics/legacyregistry/registry.go#L46-L49
		legacyregistry.DefaultGatherer,
		// For CSI operations.
		metricsManager.GetRegistry(),
	}

	pluginCapabilities, controllerCapabilities, err := ctrl.GetDriverCapabilities(grpcClient, *operationTimeout)
	if err != nil {
		klog.Fatalf("Error getting CSI driver capabilities: %s", err)
	}

	// Generate a unique ID for this provisioner
	timeStamp := time.Now().UnixNano() / int64(time.Millisecond)
	identity := strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + provisionerName
	if *enableNodeDeployment {
		identity = identity + "-" + node
	}

	factory := informers.NewSharedInformerFactory(clientset, ctrl.ResyncPeriodOfCsiNodeInformer)
	var factoryForNamespace informers.SharedInformerFactory // usually nil, only used for CSIStorageCapacity

	// -------------------------------
	// Listers
	// Create informer to prevent hit the API server for all resource request
	scLister := factory.Storage().V1().StorageClasses().Lister()
	claimLister := factory.Core().V1().PersistentVolumeClaims().Lister()

	var vaLister storagelistersv1.VolumeAttachmentLister
	if controllerCapabilities[csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME] {
		klog.Info("CSI driver supports PUBLISH_UNPUBLISH_VOLUME, watching VolumeAttachments")
		vaLister = factory.Storage().V1().VolumeAttachments().Lister()
	} else {
		klog.Info("CSI driver does not support PUBLISH_UNPUBLISH_VOLUME, not watching VolumeAttachments")
	}

	var nodeDeployment *ctrl.NodeDeployment
	if *enableNodeDeployment {
		nodeDeployment = &ctrl.NodeDeployment{
			NodeName:         node,
			ClaimInformer:    factory.Core().V1().PersistentVolumeClaims(),
			ImmediateBinding: *nodeDeploymentImmediateBinding,
			BaseDelay:        *nodeDeploymentBaseDelay,
			MaxDelay:         *nodeDeploymentMaxDelay,
		}
		nodeInfo, err := ctrl.GetNodeInfo(grpcClient, *operationTimeout)
		if err != nil {
			klog.Fatalf("Failed to get node info from CSI driver: %v", err)
		}
		nodeDeployment.NodeInfo = nodeInfo
	}

	var nodeLister listersv1.NodeLister
	var csiNodeLister storagelistersv1.CSINodeLister
	if ctrl.SupportsTopology(pluginCapabilities) {
		if nodeDeployment != nil {
			// Avoid watching in favor of fake, static objects. This is particularly relevant for
			// Node objects, which can generate significant traffic.
			csiNode := &storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeDeployment.NodeName,
				},
				Spec: storagev1.CSINodeSpec{
					Drivers: []storagev1.CSINodeDriver{
						{
							Name:   provisionerName,
							NodeID: nodeDeployment.NodeInfo.NodeId,
						},
					},
				},
			}
			node := &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeDeployment.NodeName,
				},
			}
			if nodeDeployment.NodeInfo.AccessibleTopology != nil {
				for key := range nodeDeployment.NodeInfo.AccessibleTopology.Segments {
					csiNode.Spec.Drivers[0].TopologyKeys = append(csiNode.Spec.Drivers[0].TopologyKeys, key)
				}
				node.Labels = nodeDeployment.NodeInfo.AccessibleTopology.Segments
			}
			klog.Infof("using local topology with Node = %+v and CSINode = %+v", node, csiNode)

			// We make those fake objects available to the topology code via informers which
			// never change.
			stoppedFactory := informers.NewSharedInformerFactory(clientset, 1000*time.Hour)
			csiNodes := stoppedFactory.Storage().V1().CSINodes()
			nodes := stoppedFactory.Core().V1().Nodes()
			csiNodes.Informer().GetStore().Add(csiNode)
			nodes.Informer().GetStore().Add(node)
			csiNodeLister = csiNodes.Lister()
			nodeLister = nodes.Lister()

		} else {
			csiNodeLister = factory.Storage().V1().CSINodes().Lister()
			nodeLister = factory.Core().V1().Nodes().Lister()
		}
	}

	var referenceGrantLister referenceGrantv1beta1.ReferenceGrantLister
	var gatewayFactory gatewayInformers.SharedInformerFactory
	if utilfeature.DefaultFeatureGate.Enabled(features.CrossNamespaceVolumeDataSource) {
		gatewayFactory = gatewayInformers.NewSharedInformerFactory(gatewayClient, ctrl.ResyncPeriodOfReferenceGrantInformer)
		referenceGrants := gatewayFactory.Gateway().V1beta1().ReferenceGrants()
		referenceGrantLister = referenceGrants.Lister()
	}

	// -------------------------------
	// PersistentVolumeClaims informer
	genericRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](standardflags.Configuration.RetryIntervalStart, standardflags.Configuration.RetryIntervalMax)
	claimRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](standardflags.Configuration.RetryIntervalStart, standardflags.Configuration.RetryIntervalMax)
	claimQueue := workqueue.NewTypedRateLimitingQueueWithConfig(claimRateLimiter, workqueue.TypedRateLimitingQueueConfig[string]{Name: "claims"})
	claimInformer := factory.Core().V1().PersistentVolumeClaims().Informer()

	// Setup options
	provisionerOptions := []func(*controller.ProvisionController) error{
		controller.LeaderElection(false), // Always disable leader election in provisioner lib. Leader election should be done here in the CSI provisioner level instead.
		controller.FailedProvisionThreshold(0),
		controller.FailedDeleteThreshold(0),
		controller.RateLimiter(genericRateLimiter),
		controller.Threadiness(int(*workerThreads)),
		controller.CreateProvisionedPVLimiter(workqueue.DefaultTypedControllerRateLimiter[string]()),
		controller.ClaimsInformer(claimInformer),
		controller.NodesLister(nodeLister),
		controller.RetryIntervalMax(standardflags.Configuration.RetryIntervalMax),
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.HonorPVReclaimPolicy) {
		provisionerOptions = append(provisionerOptions, controller.AddFinalizer(true))
	}

	if supportsMigrationFromInTreePluginName != "" {
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
		*immediateTopology,
		translator,
		scLister,
		csiNodeLister,
		nodeLister,
		claimLister,
		vaLister,
		referenceGrantLister,
		*extraCreateMetadata,
		*defaultFSType,
		nodeDeployment,
		*controllerPublishReadOnly,
		*preventVolumeModeConversion,
	)

	var capacityController *capacity.Controller
	if *enableCapacity {
		// Publishing storage capacity information uses its own client
		// with separate rate limiting.
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			klog.Fatalf("Failed to create client: %v", err)
		}

		namespace := os.Getenv("NAMESPACE")
		if namespace == "" {
			klog.Fatal("need NAMESPACE env variable for CSIStorageCapacity objects")
		}
		var controller *metav1.OwnerReference
		if *capacityOwnerrefLevel >= 0 {
			podName := os.Getenv("POD_NAME")
			if podName == "" {
				klog.Fatal("need POD_NAME env variable to determine CSIStorageCapacity owner")
			}
			var err error
			controller, err = owner.Lookup(config, namespace, podName,
				schema.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				},
				*capacityOwnerrefLevel)
			if err != nil {
				klog.Fatalf("look up owner(s) of pod: %v", err)
			}
			klog.Infof("using %s/%s %s as owner of CSIStorageCapacity objects", controller.APIVersion, controller.Kind, controller.Name)
		}

		var topologyInformer topology.Informer
		if nodeDeployment == nil {
			topologyRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](standardflags.Configuration.RetryIntervalStart, standardflags.Configuration.RetryIntervalMax)
			topologyInformer = topology.NewNodeTopology(
				provisionerName,
				clientset,
				factory.Core().V1().Nodes(),
				factory.Storage().V1().CSINodes(),
				workqueue.NewTypedRateLimitingQueueWithConfig(topologyRateLimiter, workqueue.TypedRateLimitingQueueConfig[string]{Name: "csitopology"}),
			)
		} else {
			var segment topology.Segment
			if nodeDeployment.NodeInfo.AccessibleTopology != nil {
				for key, value := range nodeDeployment.NodeInfo.AccessibleTopology.Segments {
					segment = append(segment, topology.SegmentEntry{Key: key, Value: value})
				}
			}
			klog.Infof("producing CSIStorageCapacity objects with fixed topology segment %s", segment)
			topologyInformer = topology.NewFixedNodeTopology(&segment)
		}
		go topologyInformer.RunWorker(ctx)

		managedByID := "external-provisioner"
		if *enableNodeDeployment {
			managedByID = getNameWithMaxLength(managedByID, node, validation.DNS1035LabelMaxLength)
		}

		// We only need objects from our own namespace. The normal factory would give
		// us an informer for the entire cluster. We can further restrict the
		// watch to just those objects with the right labels.
		factoryForNamespace = informers.NewSharedInformerFactoryWithOptions(clientset,
			ctrl.ResyncPeriodOfCsiNodeInformer,
			informers.WithNamespace(namespace),
			informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
				lo.LabelSelector = labels.Set{
					capacity.DriverNameLabel: provisionerName,
					capacity.ManagedByLabel:  managedByID,
				}.AsSelector().String()
			}),
		)

		// We use the V1 CSIStorageCapacity API if available.
		clientFactory := capacity.NewV1ClientFactory(clientset)
		cInformer := factoryForNamespace.Storage().V1().CSIStorageCapacities()

		// This invalid object is used in a v1 Create call to determine
		// based on the resulting error whether the v1 API is supported.
		invalidCapacity := &storagev1.CSIStorageCapacity{
			ObjectMeta: metav1.ObjectMeta{
				Name: "#%123-invalid-name",
			},
		}
		createdCapacity, err := clientset.StorageV1().CSIStorageCapacities(namespace).Create(ctx, invalidCapacity, metav1.CreateOptions{})
		switch {
		case err == nil:
			klog.Fatalf("creating an invalid v1.CSIStorageCapacity didn't fail as expected, got: %s", createdCapacity)
		case apierrors.IsNotFound(err):
			// We need to bridge between the v1beta1 API on the
			// server and the v1 API expected by the capacity code.
			klog.Info("using the CSIStorageCapacity v1beta1 API")
			clientFactory = capacity.NewV1beta1ClientFactory(clientset)
			cInformer = capacity.NewV1beta1InformerBridge(factoryForNamespace.Storage().V1beta1().CSIStorageCapacities())
		case apierrors.IsInvalid(err):
			klog.Info("using the CSIStorageCapacity v1 API")
		default:
			klog.Fatalf("unexpected error when checking for the V1 CSIStorageCapacity API: %v", err)
		}

		capacityRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[capacity.QueueKey](standardflags.Configuration.RetryIntervalStart, standardflags.Configuration.RetryIntervalMax)
		capacityController = capacity.NewCentralCapacityController(
			csi.NewControllerClient(grpcClient),
			provisionerName,
			clientFactory,
			// Metrics for the queue is available in the default registry.
			workqueue.NewTypedRateLimitingQueueWithConfig(capacityRateLimiter, workqueue.TypedRateLimitingQueueConfig[capacity.QueueKey]{Name: "csistoragecapacity"}),
			controller,
			managedByID,
			namespace,
			topologyInformer,
			factory.Storage().V1().StorageClasses(),
			cInformer,
			*capacityPollInterval,
			*capacityImmediateBinding,
			*operationTimeout,
		)
		legacyregistry.CustomMustRegister(capacityController)

		// Wrap Provision and Delete to detect when it is time to refresh capacity.
		csiProvisioner = capacity.NewProvisionWrapper(csiProvisioner, capacityController)
	}

	if addr != "" {
		// Start HTTP server, regardless whether we are the leader or not.
		// Register provisioner metrics manually to be able to add multiplexer in front of it
		m := libmetrics.New("controller")
		reg := prometheus.NewRegistry()
		reg.MustRegister([]prometheus.Collector{
			m.PersistentVolumeClaimProvisionTotal,
			m.PersistentVolumeClaimProvisionFailedTotal,
			m.PersistentVolumeClaimProvisionDurationSeconds,
			m.PersistentVolumeDeleteTotal,
			m.PersistentVolumeDeleteFailedTotal,
			m.PersistentVolumeDeleteDurationSeconds,
		}...)
		provisionerOptions = append(provisionerOptions, controller.MetricsInstance(m))
		gatherers = append(gatherers, reg)

		// This is similar to k8s.io/component-base/metrics HandlerWithReset
		// except that we gather from multiple sources. This is necessary
		// because both CSI metrics manager and component-base manage
		// their own registry. Probably could be avoided by making
		// CSI metrics manager a bit more flexible.
		mux.Handle(standardflags.Configuration.MetricsPath,
			promhttp.InstrumentMetricHandler(
				reg,
				promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{}),
			))

		if *enableProfile {
			klog.InfoS("Starting profiling", "endpoint", standardflags.Configuration.HttpEndpoint)

			mux.HandleFunc("/debug/pprof/", pprof.Index)
			mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		}
		go func() {
			klog.Infof("ServeMux listening at %q", addr)
			err := http.ListenAndServe(addr, mux)
			if err != nil {
				klog.Fatalf("Failed to start HTTP server at specified address (%q) and metrics path (%q): %s", addr, standardflags.Configuration.MetricsPath, err)
			}
		}()
	}

	provisionController = controller.NewProvisionController(
		ctx,
		clientset,
		provisionerName,
		csiProvisioner,
		provisionerOptions...,
	)

	csiClaimController := ctrl.NewCloningProtectionController(
		clientset,
		claimLister,
		claimInformer,
		claimQueue,
		controllerCapabilities,
	)

	// handle SIGTERM and SIGINT by cancelling the context.
	var (
		terminate       func()          // called when all controllers are finished
		controllerCtx   context.Context // shuts down all controllers on a signal
		shutdownHandler <-chan struct{} // called when the signal is received
	)

	if utilfeature.DefaultFeatureGate.Enabled(features.ReleaseLeaderElectionOnExit) {
		// ctx waits for all controllers to finish, then shuts down the whole process, incl. leader election
		ctx, terminate = context.WithCancel(ctx)
		var cancelControllerCtx context.CancelFunc
		controllerCtx, cancelControllerCtx = context.WithCancel(ctx)
		shutdownHandler = server.SetupSignalHandler()

		defer terminate()

		go func() {
			defer cancelControllerCtx()
			<-shutdownHandler
			klog.Info("Received SIGTERM or SIGINT signal, shutting down controller.")
		}()
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

		if utilfeature.DefaultFeatureGate.Enabled(features.CrossNamespaceVolumeDataSource) {
			if gatewayFactory != nil {
				gatewayFactory.Start(ctx.Done())
			}
			gatewayCacheSyncResult := gatewayFactory.WaitForCacheSync(ctx.Done())
			for _, v := range gatewayCacheSyncResult {
				if !v {
					klog.Fatalf("Failed to sync Informers for gateway!")
				}
			}
		}

		if utilfeature.DefaultFeatureGate.Enabled(features.ReleaseLeaderElectionOnExit) {
			var wg sync.WaitGroup
			if capacityController != nil {
				wg.Add(1)
				go func() {
					defer wg.Done()
					capacityController.Run(controllerCtx, int(*capacityThreads), &wg)
				}()
			}
			if csiClaimController != nil {
				wg.Add(1)
				go func() {
					defer wg.Done()
					csiClaimController.Run(controllerCtx, int(*finalizerThreads), &wg)
				}()
			}
			provisionController.ControllerWaitGroup(&wg)
			provisionController.Run(controllerCtx)
			wg.Wait()
			terminate()
		} else {
			if capacityController != nil {
				go capacityController.Run(ctx, int(*capacityThreads), nil)
			}
			if csiClaimController != nil {
				go csiClaimController.Run(ctx, int(*finalizerThreads), nil)
			}
			provisionController.Run(ctx)
		}
	}

	leaderelection.RunWithLeaderElection(
		ctx,
		config,
		standardflags.Configuration,
		run,
		strings.Replace(provisionerName, "/", "-", -1),
		mux,
		utilfeature.DefaultFeatureGate.Enabled(features.ReleaseLeaderElectionOnExit),
	)
}
