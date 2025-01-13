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

package v1beta1

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
	apisv1beta1 "sigs.k8s.io/gateway-api/apis/applyconfiguration/apis/v1beta1"
	v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	scheme "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/scheme"
)

// GatewayClassesGetter has a method to return a GatewayClassInterface.
// A group's client should implement this interface.
type GatewayClassesGetter interface {
	GatewayClasses() GatewayClassInterface
}

// GatewayClassInterface has methods to work with GatewayClass resources.
type GatewayClassInterface interface {
	Create(ctx context.Context, gatewayClass *v1beta1.GatewayClass, opts v1.CreateOptions) (*v1beta1.GatewayClass, error)
	Update(ctx context.Context, gatewayClass *v1beta1.GatewayClass, opts v1.UpdateOptions) (*v1beta1.GatewayClass, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, gatewayClass *v1beta1.GatewayClass, opts v1.UpdateOptions) (*v1beta1.GatewayClass, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.GatewayClass, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.GatewayClassList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.GatewayClass, err error)
	Apply(ctx context.Context, gatewayClass *apisv1beta1.GatewayClassApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.GatewayClass, err error)
	// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
	ApplyStatus(ctx context.Context, gatewayClass *apisv1beta1.GatewayClassApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.GatewayClass, err error)
	GatewayClassExpansion
}

// gatewayClasses implements GatewayClassInterface
type gatewayClasses struct {
	*gentype.ClientWithListAndApply[*v1beta1.GatewayClass, *v1beta1.GatewayClassList, *apisv1beta1.GatewayClassApplyConfiguration]
}

// newGatewayClasses returns a GatewayClasses
func newGatewayClasses(c *GatewayV1beta1Client) *gatewayClasses {
	return &gatewayClasses{
		gentype.NewClientWithListAndApply[*v1beta1.GatewayClass, *v1beta1.GatewayClassList, *apisv1beta1.GatewayClassApplyConfiguration](
			"gatewayclasses",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *v1beta1.GatewayClass { return &v1beta1.GatewayClass{} },
			func() *v1beta1.GatewayClassList { return &v1beta1.GatewayClassList{} }),
	}
}
