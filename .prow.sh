#! /bin/bash

. release-tools/prow.sh

# This check assumes that the current configuration uses a driver deployment
# which has been updated to use v1 APIs that aren't available in Kubernetes < 1.17.
# TODO: The check can be removed when all Prow jobs for Kubernetes < 1.17 are removed.
if ! (version_gt "${CSI_PROW_KUBERNETES_VERSION}" "1.16.255" || [ "${CSI_PROW_KUBERNETES_VERSION}" == "latest" ]); then
    filtered_tests=
    skipped_tests=
    for test in ${CSI_PROW_TESTS}; do
	case "$test" in
	    parallel | parallel | serial | parallel-alpha | serial-alpha)
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
