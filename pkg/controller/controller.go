/*
Copyright 2017 The Kubernetes Authors.

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

package controller

import (
	"github.com/golang/glog"

	"github.com/kubernetes-incubator/external-storage/lib/controller"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type csiProvisioner struct {
	client      kubernetes.Interface
	csiEndpoint string
	identity    string
	config      *rest.Config
}

var _ controller.Provisioner = &csiProvisioner{}

func NewCSIProvisioner(client kubernetes.Interface, csiEndpoint string, identity string) controller.Provisioner {
	return newCSIProvisionerInternal(client, csiEndpoint, identity)
}

func newCSIProvisionerInternal(client kubernetes.Interface, csiEndpoint string, identity string) *csiProvisioner {
	provisioner := &csiProvisioner{
		client:      client,
		csiEndpoint: csiEndpoint,
		identity:    identity,
	}
	return provisioner
}

func (p *csiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	glog.Infof("Provisioner %s Provision(..) called..")
	return nil, nil
}

func (p *csiProvisioner) Delete(volume *v1.PersistentVolume) error {
	glog.Infof("Provisioner %s Delete(..) called..")
	return nil
}
