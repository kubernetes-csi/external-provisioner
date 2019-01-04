#!/bin/bash

FAILURES=()

run_test() {
	cmd="${1}"
	echo "-- Running: ${tname} --"
	if [ ${tname} = "gotest.sh" ]; then
		"${cmd}" "$ARGS"
	else
		"${cmd}"
	fi
	sts=$?
	if [[ ${sts} -ne 0 ]]; then
		echo "failed ${cmd}"
		FAILURES+=("${cmd}")
	fi
	echo ""
}

summary() {
	if [[ ${#FAILURES[@]} -gt 0 ]]; then
		echo "ERROR: failing tests:"
		for i in "${!FAILURES[@]}"; do
			echo "  ${FAILURES[i]}"
		done
		exit 1
	else
		echo "all tests passed"
		exit 0
	fi
}

trap summary EXIT

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
ARGS="${1}"
cd "${SCRIPT_DIR}" || exit 1
for tname in $(ls tests); do
	tpath="./tests/${tname}"
	if [[ ${tpath} =~ .*\.sh$ && -f ${tpath} ]]; then
		run_test "${tpath}"
	fi
done
