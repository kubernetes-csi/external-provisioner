/*
Copyright 2019 The Kubernetes Authors.

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

package metrics

import (
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/component-base/metrics"
)

const (
	// CSICreateVolumeOperationName is the name the CSI CreateVolume operation.
	CSICreateVolumeOperationName = "CreateVolume"

	// CSIDeleteVolumeOperationName is the name the CSI DeleteVolume operation.
	CSIDeleteVolumeOperationName = "DeleteVolume"

	// Common metric strings
	subsystem             = "csi_sidecar"
	labelCSIDriverName    = "driver_name"
	labelCSIOperationName = "operation_name"
	labelGrpcStatusCode   = "grpc_status_code"

	// CSI Operation Total with status code - Count Metric
	operationsMetricName = "operations_total"
	operationsMetricHelp = "Container Storage Interface - Total count of operations with gRPC error code status"

	// CSI Operation Latency with status code - Histogram Metric
	operationsLatencyMetricName = "operations_seconds"
	operationsLatencyHelp       = "Container Storage Interface operation duration with gRPC error code status"
)

var (
	operationsLatencyBuckets = []float64{.1, .25, .5, 1, 2.5, 5, 10, 15, 25, 50, 120, 300, 600}
)

// ProvisionerMetricsManager exposes a set of operations for triggering metrics
// for this provisioner.
type ProvisionerMetricsManager interface {
	// GetRegistry() returns the metrics.KubeRegistry used by this metrics manager.
	GetRegistry() metrics.KubeRegistry

	// RecordMetrics must be called upon CSI Operation completion to record
	// the operation's metric.
	// operationName - Name of the CSI operation.
	// driverName - Name of the CSI driver against which this operation was executed.
	// operationErr - Error, if any, that resulted from execution of operation.
	// operationDuration - time it took for the operation to complete
	RecordMetrics(
		operationName string,
		driverName string,
		operationErr error,
		operationDuration time.Duration)
}

// NewProvisionerMetricsManager creates and registers metrics for for this
// provisioner and returns an object that can be used to trigger the metrics.
func NewProvisionerMetricsManager() ProvisionerMetricsManager {
	pmm := provisionerMetricsManager{
		registry: metrics.NewKubeRegistry(),
		csiOperationsMetric: metrics.NewCounterVec(
			&metrics.CounterOpts{
				Subsystem:      subsystem,
				Name:           operationsMetricName,
				Help:           operationsMetricHelp,
				StabilityLevel: metrics.ALPHA,
			},
			[]string{labelCSIDriverName, labelCSIOperationName, labelGrpcStatusCode},
		),
		csiOperationsLatencyMetric: metrics.NewHistogramVec(
			&metrics.HistogramOpts{
				Subsystem:      subsystem,
				Name:           operationsLatencyMetricName,
				Help:           operationsLatencyHelp,
				Buckets:        operationsLatencyBuckets,
				StabilityLevel: metrics.ALPHA,
			},
			[]string{labelCSIDriverName, labelCSIOperationName, labelGrpcStatusCode},
		),
	}

	pmm.registerMetrics()
	return pmm
}

var _ ProvisionerMetricsManager = &provisionerMetricsManager{}

type provisionerMetricsManager struct {
	registry                   metrics.KubeRegistry
	csiOperationsMetric        *metrics.CounterVec
	csiErrorMetric             *metrics.CounterVec
	csiOperationsLatencyMetric *metrics.HistogramVec
}

func (pmm provisionerMetricsManager) GetRegistry() metrics.KubeRegistry {
	return pmm.registry
}

func (pmm provisionerMetricsManager) RecordMetrics(
	operationName string,
	driverName string,
	operationErr error,
	operationDuration time.Duration) {
	pmm.csiOperationsLatencyMetric.WithLabelValues(
		driverName, operationName, getErrorCode(operationErr)).Observe(operationDuration.Seconds())

	pmm.csiOperationsMetric.WithLabelValues(
		driverName, operationName, getErrorCode(operationErr)).Inc()
}

func (pmm provisionerMetricsManager) registerMetrics() {
	pmm.registry.MustRegister(pmm.csiOperationsMetric)
	pmm.registry.MustRegister(pmm.csiOperationsLatencyMetric)
}

func getErrorCode(err error) string {
	if err == nil {
		return codes.OK.String()
	}

	st, ok := status.FromError(err)
	if !ok {
		// This is not gRPC error. The operation must have failed before gRPC
		// method was called, otherwise we would get gRPC error.
		return "unknown-non-grpc"
	}

	return st.Code().String()
}
