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
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/component-base/metrics/testutil"
)

func TestRecordMetrics(t *testing.T) {
	// Arrange
	pmm := NewProvisionerMetricsManager().(provisionerMetricsManager)
	operationDuration, _ := time.ParseDuration("20s")

	// Act
	pmm.RecordMetrics(
		"myOperation",        /* operationName */
		"fake.csi.driver.io", /* driverName */
		nil,                  /* operationErr */
		operationDuration /* operationDuration */)

	// Assert
	expectedMetrics := `# HELP csi_sidecar_operations_seconds [ALPHA] Container Storage Interface operation duration with gRPC error code status
		# TYPE csi_sidecar_operations_seconds histogram
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="0.1"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="0.25"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="0.5"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="1"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="2.5"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="5"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="10"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="15"} 0
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="25"} 1
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="50"} 1
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="120"} 1
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="300"} 1
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="600"} 1
		csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation",le="+Inf"} 1
		csi_sidecar_operations_seconds_sum{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation"} 20
		csi_sidecar_operations_seconds_count{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation"} 1
		# HELP csi_sidecar_operations_total [ALPHA] Container Storage Interface - Total count of operations with gRPC error code status
		# TYPE csi_sidecar_operations_total counter
		csi_sidecar_operations_total{driver_name="fake.csi.driver.io",grpc_status_code="OK",operation_name="myOperation"} 1
	`

	if err := testutil.GatherAndCompare(
		pmm.GetRegistry(), strings.NewReader(expectedMetrics)); err != nil {
		t.Fatal(err)
	}
}

func TestRecordMetrics_Negative(t *testing.T) {
	// Arrange
	pmm := NewProvisionerMetricsManager().(provisionerMetricsManager)
	operationDuration, _ := time.ParseDuration("20s")

	// Act
	pmm.RecordMetrics(
		"myOperation",        /* operationName */
		"fake.csi.driver.io", /* driverName */
		status.Error(codes.InvalidArgument, "invalid input"), /* operationErr */
		operationDuration /* operationDuration */)

	// Assert
	expectedMetrics := `# HELP csi_sidecar_operations_total [ALPHA] Container Storage Interface - Total count of operations with gRPC error code status
	# TYPE csi_sidecar_operations_total counter
	csi_sidecar_operations_total{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation"} 1
	# HELP csi_sidecar_operations_seconds [ALPHA] Container Storage Interface operation duration with gRPC error code status
	# TYPE csi_sidecar_operations_seconds histogram
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="0.1"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="0.25"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="0.5"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="1"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="2.5"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="5"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="10"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="15"} 0
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="25"} 1
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="50"} 1
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="120"} 1
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="300"} 1
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="600"} 1
	csi_sidecar_operations_seconds_bucket{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation",le="+Inf"} 1
	csi_sidecar_operations_seconds_sum{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation"} 20
	csi_sidecar_operations_seconds_total{driver_name="fake.csi.driver.io",grpc_status_code="InvalidArgument",operation_name="myOperation"} 1
	`
	if err := testutil.GatherAndCompare(
		pmm.GetRegistry(), strings.NewReader(expectedMetrics)); err != nil {
		t.Fatal(err)
	}
}
