/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"
	json "encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	apisv1 "sigs.k8s.io/gateway-api/apis/applyconfiguration/apis/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// FakeGatewayClasses implements GatewayClassInterface
type FakeGatewayClasses struct {
	Fake *FakeGatewayV1
}

var gatewayclassesResource = v1.SchemeGroupVersion.WithResource("gatewayclasses")

var gatewayclassesKind = v1.SchemeGroupVersion.WithKind("GatewayClass")

// Get takes name of the gatewayClass, and returns the corresponding gatewayClass object, and an error if there is any.
func (c *FakeGatewayClasses) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.GatewayClass, err error) {
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootGetActionWithOptions(gatewayclassesResource, name, options), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}

// List takes label and field selectors, and returns the list of GatewayClasses that match those selectors.
func (c *FakeGatewayClasses) List(ctx context.Context, opts metav1.ListOptions) (result *v1.GatewayClassList, err error) {
	emptyResult := &v1.GatewayClassList{}
	obj, err := c.Fake.
		Invokes(testing.NewRootListActionWithOptions(gatewayclassesResource, gatewayclassesKind, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.GatewayClassList{ListMeta: obj.(*v1.GatewayClassList).ListMeta}
	for _, item := range obj.(*v1.GatewayClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gatewayClasses.
func (c *FakeGatewayClasses) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchActionWithOptions(gatewayclassesResource, opts))
}

// Create takes the representation of a gatewayClass and creates it.  Returns the server's representation of the gatewayClass, and an error, if there is any.
func (c *FakeGatewayClasses) Create(ctx context.Context, gatewayClass *v1.GatewayClass, opts metav1.CreateOptions) (result *v1.GatewayClass, err error) {
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateActionWithOptions(gatewayclassesResource, gatewayClass, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}

// Update takes the representation of a gatewayClass and updates it. Returns the server's representation of the gatewayClass, and an error, if there is any.
func (c *FakeGatewayClasses) Update(ctx context.Context, gatewayClass *v1.GatewayClass, opts metav1.UpdateOptions) (result *v1.GatewayClass, err error) {
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateActionWithOptions(gatewayclassesResource, gatewayClass, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeGatewayClasses) UpdateStatus(ctx context.Context, gatewayClass *v1.GatewayClass, opts metav1.UpdateOptions) (result *v1.GatewayClass, err error) {
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceActionWithOptions(gatewayclassesResource, "status", gatewayClass, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}

// Delete takes name of the gatewayClass and deletes it. Returns an error if one occurs.
func (c *FakeGatewayClasses) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(gatewayclassesResource, name, opts), &v1.GatewayClass{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGatewayClasses) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewRootDeleteCollectionActionWithOptions(gatewayclassesResource, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1.GatewayClassList{})
	return err
}

// Patch applies the patch and returns the patched gatewayClass.
func (c *FakeGatewayClasses) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.GatewayClass, err error) {
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceActionWithOptions(gatewayclassesResource, name, pt, data, opts, subresources...), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied gatewayClass.
func (c *FakeGatewayClasses) Apply(ctx context.Context, gatewayClass *apisv1.GatewayClassApplyConfiguration, opts metav1.ApplyOptions) (result *v1.GatewayClass, err error) {
	if gatewayClass == nil {
		return nil, fmt.Errorf("gatewayClass provided to Apply must not be nil")
	}
	data, err := json.Marshal(gatewayClass)
	if err != nil {
		return nil, err
	}
	name := gatewayClass.Name
	if name == nil {
		return nil, fmt.Errorf("gatewayClass.Name must be provided to Apply")
	}
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceActionWithOptions(gatewayclassesResource, *name, types.ApplyPatchType, data, opts.ToPatchOptions()), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeGatewayClasses) ApplyStatus(ctx context.Context, gatewayClass *apisv1.GatewayClassApplyConfiguration, opts metav1.ApplyOptions) (result *v1.GatewayClass, err error) {
	if gatewayClass == nil {
		return nil, fmt.Errorf("gatewayClass provided to Apply must not be nil")
	}
	data, err := json.Marshal(gatewayClass)
	if err != nil {
		return nil, err
	}
	name := gatewayClass.Name
	if name == nil {
		return nil, fmt.Errorf("gatewayClass.Name must be provided to Apply")
	}
	emptyResult := &v1.GatewayClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceActionWithOptions(gatewayclassesResource, *name, types.ApplyPatchType, data, opts.ToPatchOptions(), "status"), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GatewayClass), err
}
