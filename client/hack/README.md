# Scripts User Guide

This README documents:
* What update-crd.sh and update-generated-code.sh do
* When and how to use them

## update-generated-code.sh

This is the script to update clientset/informers/listers and API deepcopy code using [code-generator](https://github.com/kubernetes/code-generator).

Make sure to run this script after making changes to /client/apis/volumepopulator/v1beta1/types.go.

Pre-requisites for running update-generated-code.sh:

* GOPATH=~/go

* Ensure external-provisioner repository is at ~/go/src/github.com/kubernetes-csi/external-provisioner

* git clone https://github.com/kubernetes/code-generator.git under ~/go/src/k8s.io

* git checkout to version v0.19.0
```bash
git checkout v0.19.0
```

* Ensure the path exist ${GOPATH}/src/k8s.io/code-generator/generate-groups.sh

Run: ./hack/update-generated-code.sh from the client directory.

Once you run the script, you will get an output as follows:
```bash
Generating deepcopy funcs
Generating clientset for volumepopulator:v1beta1 at github.com/kubernetes-csi/external-provisioner/client/clientset
Generating listers for volumepopulator:v1beta1 at github.com/kubernetes-csi/external-provisioner/client/listers
Generating informers for volumepopulator:v1beta1 at github.com/kubernetes-csi/external-provisioner/client/informers
```


## update-crd.sh

This is the script to update CRD yaml files under /client/config/crd/ based on types.go file.

Make sure to run this script after making changes to /client/apis/volumepopulator/v1beta1/types.go.

Follow these steps to update the CRD:

* Run ./hack/update-crd.sh from client directory, new yaml files should have been created under ./config/crd/
