# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

IMAGE = k8scsi/csi-provisioner

VERSION :=
TAG := $(shell git describe --abbrev=0 --tags HEAD 2>/dev/null)
COMMIT := $(shell git rev-parse HEAD)
ifeq ($(TAG),)
    VERSION := latest
else
    ifeq ($(COMMIT), $(shell git rev-list -n1 $(TAG)))
        VERSION := $(TAG)
    else
        VERSION := $(TAG)-$(COMMIT)
    endif
endif

container: build quick-container
.PHONY: container

clean:
	rm -rf bin
.PHONY: clean

format:
	 gofmt -w -s ./
.PHONY: format

deps:
	dep ensure -update
.PHONY: deps

quick-container:
	docker build -t $(IMAGE):$(VERSION) .
.PHONY: quick-container

push-container:
	docker push $(IMAGE):$(VERSION)
.PHONY: push-container

provisioner:
	mkdir -p bin
	go install github.com/kubernetes-csi/external-provisioner/cmd/csi-provisioner
	cp ${GOPATH}/bin/csi-provisioner bin/csi-provisioner
.PHONY: provisioner

all build: provisioner

.PHONY: all build
