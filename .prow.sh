#! /bin/bash

. release-tools/prow.sh

# External-provisioner is updated to use VolumeSnapshot v1 APIs. As a result,
# tesing on Kubernetes version < 1.20 are no longer supported because they use
# v1beta1 or earlier VolumeSnapshot APIs.
# TODO: The check can be removed when all Prow jobs for Kubernetes < 1.20 are removed.
if ! (version_gt "${CSI_PROW_KUBERNETES_VERSION}" "1.19.255" || [ "${CSI_PROW_KUBERNETES_VERSION}" == "latest" ]); then
    filtered_tests=
    skipped_tests=
    for test in ${CSI_PROW_TESTS}; do
	case "$test" in
	    parallel | serial | parallel-alpha | serial-alpha)
		skipped_tests="$skipped_tests $test"
		;;
	    *)
		filtered_tests="$filtered_tests $test"
		;;
	esac
    done
    if [ "$skipped_tests" ]; then
	info "Testing on Kubernetes ${CSI_PROW_KUBERNETES_VERSION} is no longer supported. Skipping CSI_PROW_TESTS: $skipped_tests."
	CSI_PROW_TESTS="$filtered_tests"
    fi
fi

main
