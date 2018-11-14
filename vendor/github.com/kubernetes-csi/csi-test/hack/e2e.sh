#!/bin/bash

CSI_ENDPOINTS="tcp://127.0.0.1:9998"
CSI_ENDPOINTS="$CSI_ENDPOINTS /tmp/e2e-csi-sanity.sock"
CSI_ENDPOINTS="$CSI_ENDPOINTS unix:///tmp/e2e-csi-sanity.sock"

go get -u github.com/thecodeteam/gocsi/mock
cd cmd/csi-sanity
  make clean install || exit 1
cd ../..

for endpoint in $CSI_ENDPOINTS ; do
	if ! echo $endpoint | grep tcp > /dev/null 2>&1 ; then
		rm -f $endpoint
	fi

	CSI_ENDPOINT=$endpoint mock &
	pid=$!

	csi-sanity $@ --ginkgo.skip=MOCKERRORS --csi.endpoint=$endpoint ; ret=$?
	kill -9 $pid

	if ! echo $endpoint | grep tcp > /dev/null 2>&1 ; then
		rm -f $endpoint
	fi

	if [ $ret -ne 0 ] ; then
		exit $ret
	fi
done

exit 0
