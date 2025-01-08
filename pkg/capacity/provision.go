/*
Copyright 2021 The Kubernetes Authors.

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

package capacity

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v11/controller"
)

type provisionWrapper struct {
	controller.Provisioner
	c *Controller
}

var _ controller.Provisioner = &provisionWrapper{}
var _ controller.BlockProvisioner = &provisionWrapper{}
var _ controller.Qualifier = &provisionWrapper{}

func NewProvisionWrapper(p controller.Provisioner, c *Controller) controller.Provisioner {
	return &provisionWrapper{
		Provisioner: p,
		c:           c,
	}
}

func (p *provisionWrapper) Provision(ctx context.Context, options controller.ProvisionOptions) (pv *v1.PersistentVolume, state controller.ProvisioningState, err error) {
	pv, state, err = p.Provisioner.Provision(ctx, options)
	if err == nil && pv != nil {
		if pv.Spec.NodeAffinity != nil {
			// If we know where the volume was
			// provisioned, then refresh all objects in
			// that topology. This should cover all
			// relevant objects.
			//
			// As with the other cases, this is just a
			// heuristic that tries to refresh those
			// objects sooner which probably have
			// changed. We cannot be sure that other
			// segments were not affected, but that will
			// be covered by the periodic refresh.
			p.c.refreshTopology(*pv.Spec.NodeAffinity)
		} else if options.StorageClass != nil {
			// Fall back to refresh by storage class.
			// This is useful for a driver with network
			// attached storage (= no topology) where
			// storage class parameters select certain
			// distinct storage pools ("fast" for SSD,
			// "slow" for HD).
			p.c.refreshSC(options.StorageClass.Name)
		}
	} else if state != controller.ProvisioningNoChange {
		// Unsuccessful provisioning might also be a reason why
		// we have to refresh. We could try to identify the topology
		// via the selected node (if there is any), but more important
		// and easier is to refresh the objects for the storage
		// class. That will help choosing a node for the volume
		// that couldn't be created.
		if options.StorageClass != nil {
			p.c.refreshSC(options.StorageClass.Name)
		}
	}
	return
}

func (p *provisionWrapper) Delete(ctx context.Context, pv *v1.PersistentVolume) (err error) {
	err = p.Provisioner.Delete(ctx, pv)
	if err == nil && pv.Spec.NodeAffinity != nil {
		// We don't know the storage class, but the
		// topology is even better.
		p.c.refreshTopology(*pv.Spec.NodeAffinity)
	}
	return
}

func (p *provisionWrapper) SupportsBlock(ctx context.Context) bool {
	if blockProvisioner, ok := p.Provisioner.(controller.BlockProvisioner); ok {
		return blockProvisioner.SupportsBlock(ctx)
	}
	return false
}

func (p *provisionWrapper) ShouldProvision(ctx context.Context, claim *v1.PersistentVolumeClaim) bool {
	if qualifier, ok := p.Provisioner.(controller.Qualifier); ok {
		return qualifier.ShouldProvision(ctx, claim)
	}
	return false
}
