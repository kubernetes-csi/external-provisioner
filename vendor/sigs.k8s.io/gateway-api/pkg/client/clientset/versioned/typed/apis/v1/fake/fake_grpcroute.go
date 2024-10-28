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

// FakeGRPCRoutes implements GRPCRouteInterface
type FakeGRPCRoutes struct {
	Fake *FakeGatewayV1
	ns   string
}

var grpcroutesResource = v1.SchemeGroupVersion.WithResource("grpcroutes")

var grpcroutesKind = v1.SchemeGroupVersion.WithKind("GRPCRoute")

// Get takes name of the gRPCRoute, and returns the corresponding gRPCRoute object, and an error if there is any.
func (c *FakeGRPCRoutes) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.GRPCRoute, err error) {
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(grpcroutesResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}

// List takes label and field selectors, and returns the list of GRPCRoutes that match those selectors.
func (c *FakeGRPCRoutes) List(ctx context.Context, opts metav1.ListOptions) (result *v1.GRPCRouteList, err error) {
	emptyResult := &v1.GRPCRouteList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(grpcroutesResource, grpcroutesKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.GRPCRouteList{ListMeta: obj.(*v1.GRPCRouteList).ListMeta}
	for _, item := range obj.(*v1.GRPCRouteList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gRPCRoutes.
func (c *FakeGRPCRoutes) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(grpcroutesResource, c.ns, opts))

}

// Create takes the representation of a gRPCRoute and creates it.  Returns the server's representation of the gRPCRoute, and an error, if there is any.
func (c *FakeGRPCRoutes) Create(ctx context.Context, gRPCRoute *v1.GRPCRoute, opts metav1.CreateOptions) (result *v1.GRPCRoute, err error) {
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(grpcroutesResource, c.ns, gRPCRoute, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}

// Update takes the representation of a gRPCRoute and updates it. Returns the server's representation of the gRPCRoute, and an error, if there is any.
func (c *FakeGRPCRoutes) Update(ctx context.Context, gRPCRoute *v1.GRPCRoute, opts metav1.UpdateOptions) (result *v1.GRPCRoute, err error) {
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(grpcroutesResource, c.ns, gRPCRoute, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeGRPCRoutes) UpdateStatus(ctx context.Context, gRPCRoute *v1.GRPCRoute, opts metav1.UpdateOptions) (result *v1.GRPCRoute, err error) {
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(grpcroutesResource, "status", c.ns, gRPCRoute, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}

// Delete takes name of the gRPCRoute and deletes it. Returns an error if one occurs.
func (c *FakeGRPCRoutes) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(grpcroutesResource, c.ns, name, opts), &v1.GRPCRoute{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGRPCRoutes) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(grpcroutesResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1.GRPCRouteList{})
	return err
}

// Patch applies the patch and returns the patched gRPCRoute.
func (c *FakeGRPCRoutes) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.GRPCRoute, err error) {
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(grpcroutesResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied gRPCRoute.
func (c *FakeGRPCRoutes) Apply(ctx context.Context, gRPCRoute *apisv1.GRPCRouteApplyConfiguration, opts metav1.ApplyOptions) (result *v1.GRPCRoute, err error) {
	if gRPCRoute == nil {
		return nil, fmt.Errorf("gRPCRoute provided to Apply must not be nil")
	}
	data, err := json.Marshal(gRPCRoute)
	if err != nil {
		return nil, err
	}
	name := gRPCRoute.Name
	if name == nil {
		return nil, fmt.Errorf("gRPCRoute.Name must be provided to Apply")
	}
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(grpcroutesResource, c.ns, *name, types.ApplyPatchType, data, opts.ToPatchOptions()), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeGRPCRoutes) ApplyStatus(ctx context.Context, gRPCRoute *apisv1.GRPCRouteApplyConfiguration, opts metav1.ApplyOptions) (result *v1.GRPCRoute, err error) {
	if gRPCRoute == nil {
		return nil, fmt.Errorf("gRPCRoute provided to Apply must not be nil")
	}
	data, err := json.Marshal(gRPCRoute)
	if err != nil {
		return nil, err
	}
	name := gRPCRoute.Name
	if name == nil {
		return nil, fmt.Errorf("gRPCRoute.Name must be provided to Apply")
	}
	emptyResult := &v1.GRPCRoute{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(grpcroutesResource, c.ns, *name, types.ApplyPatchType, data, opts.ToPatchOptions(), "status"), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.GRPCRoute), err
}
