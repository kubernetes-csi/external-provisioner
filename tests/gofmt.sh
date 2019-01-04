#!/bin/bash

GOFMTCHECK="$(command -v gofmt 2>/dev/null)"

if [[ -z "${GOFMTCHECK}" ]]; then
	echo "warning: could not find gofmt ... will skip checks" >&2
	exit 0
fi

ERROR=0
for pkg in $(go list ./... | grep -v '/vendor/'); do
	dir="$GOPATH/src/$pkg"
	FILES=$(gofmt -l -e "$dir"/*.go)
	if [ ${#FILES} -ne 0 ]; then
		for file in $FILES; do
			echo "$file"
		done
		ERROR=1
	fi
done

if [ $ERROR -ne 0 ]; then
	exit 1
fi
