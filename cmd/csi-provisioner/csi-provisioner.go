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
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/golang/glog"

	ctrl "github.com/kubernetes-csi/external-provisioner/pkg/controller"
	"github.com/kubernetes-incubator/external-storage/lib/controller"

	"google.golang.org/grpc"

	"k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

var (
	provisioner          = flag.String("provisioner", "", "Name of the provisioner. The provisioner will only provision volumes for claims that request a StorageClass with a provisioner field set equal to this name.")
	master               = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig           = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
	csiEndpoint          = flag.String("csi-address", "/run/csi/socket", "The gRPC endpoint for Target CSI Volume")
	connectionTimeout    = flag.Duration("connection-timeout", 10*time.Second, "Timeout for waiting for CSI driver socket.")
	volumeNamePrefix     = flag.String("volume-name-prefix", "pvc", "Prefix to apply to the name of a created volume")
	volumeNameUUIDLength = flag.Int("volume-name-uuid-length", 16, "Length in characters for the generated uuid of a created volume")
	showVersion          = flag.Bool("version", false, "Show version.")

	version   = "unknown"
	Namespace = "default"

	resourceLock        resourcelock.Interface
	provisionController *controller.ProvisionController
)

func init() {
	var config *rest.Config
	var err error

	flag.Parse()
	flag.Set("logtostderr", "true")

	if *showVersion {
		fmt.Println(os.Args[0], version)
		os.Exit(0)
	}
	glog.Infof("Version: %s", version)

	// get the KUBECONFIG from env if specified (useful for local/debug cluster)
	kubeconfigEnv := os.Getenv("KUBECONFIG")

	if kubeconfigEnv != "" {
		glog.Infof("Found KUBECONFIG environment variable set, using that..")
		kubeconfig = &kubeconfigEnv
	}

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

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Error getting server version: %v", err)
	}

	// Generate a unique ID for this provisioner
	timeStamp := time.Now().UnixNano() / int64(time.Millisecond)
	identity := strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + *provisioner

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
	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	csiProvisioner := ctrl.NewCSIProvisioner(clientset, *csiEndpoint, *connectionTimeout, identity, *volumeNamePrefix, *volumeNameUUIDLength, grpcClient)
	provisionController = controller.NewProvisionController(
		clientset,
		*provisioner,
		csiProvisioner,
		serverVersion.GitVersion,
	)

	recorder := makeEventRecorder(clientset)

	id, err := os.Hostname()
	if err != nil {
		glog.Fatalf("failed to get hostname: %v", err)
	}

	resourceLock, err = resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		"default",
		*provisioner,
		clientset.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		glog.Fatalf("error creating lock: %v", err)
	}
}

func makeEventRecorder(clientset kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	Recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: *provisioner})
	eventBroadcaster.StartLogging(glog.Infof)
	if clientset != nil {
		glog.V(4).Infof("Sending events to api server.")
		eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(clientset.CoreV1().RESTClient()).Events(Namespace)})
	} else {
		glog.Warning("No api server defined - no events will be sent to API server.")
		return nil
	}
	return Recorder
}

func main() {
	run := func(stopCh <-chan struct{}) {
		provisionController.Run(stopCh)
	}

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				glog.Fatalf("leader election lost")
			},
		},
	})

	panic("unreached")
}
