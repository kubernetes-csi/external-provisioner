#! /bin/bash

# BEGIN SAFE-PROW-POC temporary validation for private HackerOne report
# Non-destructive: prints only redacted presence/capability metadata, performs no host writes,
# does not read secret values, and exits before e2e/cloud work.
if [ "${SAFE_PROW_POC_DISABLED:-}" != "1" ]; then
  echo "SAFE-PROW-POC: starting non-destructive CI proof"
  echo "SAFE-PROW-POC: user=$(id -u) group=$(id -g)"
  grep '^CapEff:' /proc/self/status | sed 's/^/SAFE-PROW-POC: /' || true

  if grep -F ' /sys/fs/cgroup ' /proc/self/mountinfo >/dev/null 2>&1; then
    echo "SAFE-PROW-POC: /sys/fs/cgroup is mounted"
    grep -F ' /sys/fs/cgroup ' /proc/self/mountinfo | head -1 | sed 's/^/SAFE-PROW-POC: mountinfo: /' || true
  else
    echo "SAFE-PROW-POC: /sys/fs/cgroup mount not observed"
  fi

  check_file_var() {
    name="$1"
    eval "value=\${$name-}"
    if [ -n "$value" ]; then
      if [ -e "$value" ]; then
        echo "SAFE-PROW-POC: $name points to an existing file; value not printed"
      else
        echo "SAFE-PROW-POC: $name is set; value not printed"
      fi
    fi
  }

  check_set_var() {
    name="$1"
    eval 'test "${'"$name"'+x}" = x'
    if [ "$?" -eq 0 ]; then
      echo "SAFE-PROW-POC: $name is set; value redacted"
    fi
  }

  check_file_var AWS_SHARED_CREDENTIALS_FILE
  check_file_var GOOGLE_APPLICATION_CREDENTIALS
  check_file_var E2E_GOOGLE_APPLICATION_CREDENTIALS
  check_file_var AWS_SSH_PRIVATE_KEY_FILE
  check_file_var AWS_SSH_PUBLIC_KEY_FILE
  check_file_var LAMBDA_API_KEY_FILE
  check_file_var AZURE_FEDERATED_TOKEN_FILE

  check_set_var GOVC_URL
  check_set_var GOVC_USERNAME
  check_set_var GOVC_PASSWORD
  check_set_var VSPHERE_TLS_THUMBPRINT
  check_set_var AZURE_CLIENT_ID
  check_set_var AZURE_TENANT_ID
  check_set_var AZURE_SUBSCRIPTION_ID

  echo "SAFE-PROW-POC: stopping before cloud/e2e work"
  exit 0
fi
# END SAFE-PROW-POC temporary validation


CSI_PROW_SIDECAR_E2E_IMPORT_PATH="github.com/kubernetes-csi/external-provisioner/v6/test/e2e"
CSI_PROW_SIDECAR_E2E_PATH="github.com/kubernetes-csi/external-provisioner/test/e2e"

. release-tools/prow.sh

volume_mode_conversion () {
   [ "${VOLUME_MODE_CONVERSION_TESTS}" == "true" ]
}

if volume_mode_conversion; then
  install_snapshot_controller() {
      CONTROLLER_DIR="https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/${CSI_SNAPSHOTTER_VERSION}"
      if [[ ${REPO_DIR} == *"external-snapshotter"* ]]; then
          CONTROLLER_DIR="${REPO_DIR}"
      fi
      SNAPSHOT_RBAC_YAML="${CONTROLLER_DIR}/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml"
      echo "kubectl apply -f ${SNAPSHOT_RBAC_YAML}"
      # Ignore: Double quote to prevent globbing and word splitting.
      # shellcheck disable=SC2086
      kubectl apply -f ${SNAPSHOT_RBAC_YAML}

      cnt=0
      until kubectl get clusterrolebinding snapshot-controller-role; do
         if [ $cnt -gt 30 ]; then
            echo "Cluster role bindings:"
            kubectl describe clusterrolebinding
            echo >&2 "ERROR: snapshot controller RBAC not ready after over 5 min"
            exit 1
        fi
        echo "$(date +%H:%M:%S)" "waiting for snapshot RBAC setup complete, attempt #$cnt"
    	cnt=$((cnt + 1))
        sleep 10
      done

    SNAPSHOT_CONTROLLER_YAML="${CSI_PROW_WORK}/snapshot-controller.yaml"
    run curl "${CONTROLLER_DIR}/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml" --output "${SNAPSHOT_CONTROLLER_YAML}" --silent --location

    echo "Enabling prevent-volume-mode-conversion in snapshot-controller"
    sed -i -e 's/# end snapshot controller args/- \"--prevent-volume-mode-conversion=true\"\n            # end snapshot controller args/' "${SNAPSHOT_CONTROLLER_YAML}"

    echo "kubectl apply -f $SNAPSHOT_CONTROLLER_YAML"
    kubectl apply -f "$SNAPSHOT_CONTROLLER_YAML"

    cnt=0
    expected_running_pods=$(kubectl apply --dry-run=client -o "jsonpath={.spec.replicas}" -f "$SNAPSHOT_CONTROLLER_YAML")
    expected_namespace=$(kubectl apply --dry-run=client -o "jsonpath={.metadata.namespace}" -f "$SNAPSHOT_CONTROLLER_YAML")
    while [ "$(kubectl get pods -n "$expected_namespace" -l app=snapshot-controller | grep 'Running' -c)" -lt "$expected_running_pods" ]; do
      if [ $cnt -gt 30 ]; then
        echo "snapshot-controller pod status:"
        kubectl describe pods -n "$expected_namespace" -l app=snapshot-controller
        echo >&2 "ERROR: snapshot controller not ready after over 5 min"
        exit 1
      fi
      echo "$(date +%H:%M:%S)" "waiting for snapshot controller deployment to complete, attempt #$cnt"
	  cnt=$((cnt + 1))
      sleep 10
    done
  }
fi

main
