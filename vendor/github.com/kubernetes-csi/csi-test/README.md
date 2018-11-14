[![Build Status](https://travis-ci.org/kubernetes-csi/csi-test.svg?branch=master)](https://travis-ci.org/kubernetes-csi/csi-test)
# csi-test
csi-test houses packages and libraries to help test CSI client and plugins.

## For Container Orchestration Unit Tests
CO developers can use this framework to create drivers based on the
[Golang mock](https://github.com/golang/mock) framework. Please see
[co_test.go](test/co_test.go) for an example.

## For CSI Driver Unit Tests
To test drivers please take a look at [pkg/sanity](https://github.com/kubernetes-csi/csi-test/tree/master/pkg/sanity)

### Note

* Only Golang 1.9+ supported. See [gRPC issue](https://github.com/grpc/grpc-go/issues/711#issuecomment-326626790)
