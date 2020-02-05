[![Build Status](https://travis-ci.org/kubernetes-csi/external-provisioner.svg?branch=master)](https://travis-ci.org/kubernetes-csi/external-provisioner)

# CSI provisioner

The external-provisioner is a sidecar container that dynamically provisions volumes by calling `ControllerCreateVolume` and `ControllerDeleteVolume` functions of CSI drivers. It is necessary because internal persistent volume controller running in Kubernetes controller-manager does not have any direct interfaces to CSI drivers.

## Overview
The external-provisioner is an external controller that monitors `PersistentVolumeClaim` objects created by user and creates/deletes volumes for them. Full design can be found at Kubernetes proposal at [container-storage-interface.md](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md)

## Compatibility

This information reflects the head of this branch.

| Compatible with CSI Version                                                                | Container Image                | Recommended K8s Version |
| ------------------------------------------------------------------------------------------ | -------------------------------| --------------- |
| [CSI Spec v1.0.0](https://github.com/container-storage-interface/spec/releases/tag/v1.0.0) | quay.io/k8scsi/csi-provisioner | 1.17            |

## Feature status

Various external-provisioner releases come with different alpha / beta features. Check `--help` output for alpha/beta features in each release.

Following table reflects the head of this branch.

| Feature        | Status  | Default | Description                                                                                   | Provisioner Feature Gate Required |
| -------------- | ------- | ------- | --------------------------------------------------------------------------------------------- | --------------------------------- |
| Topology       | Beta    | Off     | [Topology aware dynamic provisioning](https://kubernetes-csi.github.io/docs/topology.html)  (requires kubelet 1.14 on nodes). | Yes |
| Cloning        | Beta    | On      | [Cloning](https://kubernetes-csi.github.io/docs/volume-cloning.html). | No |
| Snapshots      | Beta    | On      | [Snapshots and Restore](https://kubernetes-csi.github.io/docs/snapshot-restore-feature.html). | No |
| CSIMigration   | Beta    | On      | [Migrating in-tree volume plugins to CSI](https://kubernetes.io/docs/concepts/storage/volumes/#csi-migration). | No |

All other external-provisioner features and the external-provisioner itself is considered GA and fully supported.

## Usage

It is necessary to create a new service account and give it enough privileges to run the external-provisioner, see `deploy/kubernetes/rbac.yaml`. The provisioner is then deployed as single Deployment as illustrated below:

```sh
kubectl create deploy/kubernetes/deployment.yaml
```

The external-provisioner may run in the same pod with other external CSI controllers such as the external-attacher, external-snapshotter and/or external-resizer.

Note that the external-provisioner does not scale with more replicas. Only one external-provisioner is elected as leader and running. The others are waiting for the leader to die. They re-elect a new active leader in ~15 seconds after death of the old leader.

### Command line options

#### Recommended optional arguments
* `--csi-address <path to CSI socket>`: This is the path to the CSI driver socket inside the pod that the external-provisioner container will use to issue CSI operations (`/run/csi/socket` is used by default).

* `--leader-election`: Enables leader election. This is mandatory when there are multiple replicas of the same external-provisioner running for one CSI driver. Only one of them may be active (=leader). A new leader will be re-elected when current leader dies or becomes unresponsive for ~15 seconds.

* `--leader-election-namespace`: Namespace where leader election object will be created. It is recommended that this parameter is populated from Kubernetes DownwardAPI with the namespace where the external-provisioner runs in.

* `--timeout <duration>`: Timeout of all calls to CSI driver. It should be set to value that accommodates majority of `ControllerCreateVolume` and `ControllerDeleteVolume` calls. See [CSI error and timeout handling](#csi-error-and-timeout-handling) for details. 15 seconds is used by default.

* `--retry-interval-start <duration>`: Initial retry interval of failed provisioning or deletion. It doubles with each failure, up to `--retry-interval-max` and then it stops increasing. Default value is 1 second. See [CSI error and timeout handling](#csi-error-and-timeout-handling) for details.

* `--retry-interval-max <duration>`: Maximum retry interval of failed provisioning or deletion. Default value is 5 minutes. See [CSI error and timeout handling](#csi-error-and-timeout-handling) for details.

* `--worker-threads <num>`: Number of simultaneously running `ControllerCreateVolume` and `ControllerDeleteVolume` operations. Default value is `100`.

* `--metrics-address`: The TCP network address where the prometheus metrics endpoint will run (example: `:8080` which corresponds to port 8080 on local host). The default is empty string, which means metrics endpoint is disabled.

* `--metrics-path`: The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.

#### Other recognized arguments
* `--feature-gates <gates>`: A set of comma separated `<feature-name>=<true|false>` pairs that describe feature gates for alpha/experimental features. See [list of features](#feature-status) or `--help` output for list of recognized features. Example: `--feature-gates Topology=true` to enable Topology feature that's disabled by default.

* `--strict-topology`: This controls what topology information is passed to `CreateVolumeRequest.AccessibilityRequirements` in case of delayed binding. See [the table below](#topology-support) for an explanation how this option changes the result. This option has no effect if either `Topology` feature is disabled or `Immediate` volume binding mode is used.

* `--kubeconfig <path>`: Path to Kubernetes client configuration that the external-provisioner uses to connect to Kubernetes API server. When omitted, default token provided by Kubernetes will be used. This option is useful only when the external-provisioner does not run as a Kubernetes pod, e.g. for debugging. Either this or `--master` needs to be set if the external-provisioner is being run out of cluster.

* `--master <url>`: Master URL to build a client config from. When omitted, default token provided by Kubernetes will be used. This option is useful only when the external-provisioner does not run as a Kubernetes pod, e.g. for debugging. Either this or `--kubeconfig` needs to be set if the external-provisioner is being run out of cluster.

* `--volume-name-prefix <prefix>`: Prefix of PersistentVolume names created by the external-provisioner. Default value is "pvc", i.e. created PersistentVolume objects will have name `pvc-<uuid>`.

* `--volume-name-uuid-length`: Length of UUID to be added to `--volume-name-prefix`. Default behavior is to NOT truncate the UUID.

* `--version`: Prints current external-provisioner version and quits.

* All glog / klog arguments are supported, such as `-v <log level>` or `-alsologtostderr`.

#### Deprecated arguments
* `--connection-timeout <duration>`: This option was used to limit establishing connection to CSI driver. Currently, the option does not have any effect and the external-provisioner tries to connect to CSI driver socket indefinitely. It is recommended to run ReadinessProbe on the driver to ensure that the driver comes up in reasonable time.

* `--provisioner`: This option was used to set a provisioner name to look for in the StorageClass. Currently, the option does not have any effect and the external-provisioner uses the CSI driver name.

### Topology support
When `Topology` feature is enabled and the driver specifies `VOLUME_ACCESSIBILITY_CONSTRAINTS` in its plugin capabilities, external-provisioner prepares `CreateVolumeRequest.AccessibilityRequirements` while calling `Controller.CreateVolume`. The driver has to consider these topology constraints while creating the volume. Below table shows how these `AccessibilityRequirements` are prepared:

[Delayed binding](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode) | Strict topology | [Allowed topologies](https://kubernetes.io/docs/concepts/storage/storage-classes/#allowed-topologies) | [Resulting accessability requirements](https://github.com/container-storage-interface/spec/blob/master/spec.md#createvolume)
:---: |:---:|:---:|:---|
Yes | Yes | Irrelevant | `Requisite` = `Preferred` = Selected node topology
Yes | No  | No | `Requisite` = Aggregated cluster topology<br>`Preferred` = `Requisite` with selected node topology as first element
Yes | No | Yes | `Requisite` = Allowed topologies<br>`Preferred` = `Requisite` with selected node topology as first element
No | Irrelevant | No | `Requisite` = Aggregated cluster topology<br>`Preferred` = `Requisite` with randomly selected node topology as first element
No | Irrelevant | Yes | `Requisite` = Allowed topologies<br>`Preferred` = `Requisite` with randomly selected node topology as first element

### CSI error and timeout handling
The external-provisioner invokes all gRPC calls to CSI driver with timeout provided by `--timeout` command line argument (15 seconds by default).

Correct timeout value and number of worker threads depends on the storage backend and how quickly it is able to processes `ControllerCreateVolume` and `ControllerDeleteVolume` calls. The value should be set to accommodate majority of them. It is fine if some calls time out - such calls will be retried after exponential backoff (starting with 1s by default), however, this backoff will introduce delay when the call times out several times for a single volume.

Frequency of `ControllerCreateVolume` and `ControllerDeleteVolume` retries can be configured by `--retry-interval-start` and `--retry-interval-max` parameters. The external-provisioner starts retries with `retry-interval-start` interval (1s by default) and doubles it with each failure until it reaches `retry-interval-max` (5 minutes by default). The external provisioner stops increasing the retry interval when it reaches `retry-interval-max`, however, it still retries provisioning/deletion of a volume until it's provisioned. The external-provisioner keeps its own number of provisioning/deletion failures for each volume.

The external-provisioner can invoke up to `--worker-threads` (100 by default) `ControllerCreateVolume` **and** up to `--worker-threads` `ControllerDeleteVolume` calls in parallel, i.e. these two calls are counted separately. The external-provisioner assumes that the storage backend can cope with such high number of parallel requests and that the requests are handled in relatively short time (ideally sub-second). Lower value should be used for storage backends that expect slower processing related to newly created / deleted volumes or can handle lower amount of parallel calls.

Details of error handling of individual CSI calls:
* `ControllerCreateVolume`: The call might have timed out just before the driver provisioned a volume and was sending a response. From that reason, timeouts from `ControllerCreateVolume` is considered as "*volume may be provisioned*" or "*volume is being provisioned in the background*." The external-provisioner will retry calling `ControllerCreateVolume` after exponential backoff until it gets either successful response or final (non-timeout) error that the volume cannot be created.
* `ControllerDeleteVolume`: This is similar to `ControllerCreateVolume`, The external-provisioner will retry calling `ControllerDeleteVolume` with exponential backoff after timeout until it gets either successful response or a final error that the volume cannot be deleted.
* `Probe`: The external-provisioner retries calling Probe until the driver reports it's ready. It retries also when it receives timeout from `Probe` call. The external-provisioner has no limit of retries. It is expected that ReadinessProbe on the driver container will catch case when the driver takes too long time to get ready.
* `GetPluginInfo`, `GetPluginCapabilitiesRequest`, `ControllerGetCapabilities`: The external-provisioner expects that these calls are quick and does not retry them on any error, including timeout. Instead, it assumes that the driver is faulty and exits. Note that Kubernetes will likely start a new provisioner container and it will start with `Probe` call.

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack channel](https://kubernetes.slack.com/messages/sig-storage)
- [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-sig-storage)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
