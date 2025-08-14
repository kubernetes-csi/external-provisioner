package controller

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// TODO: This function should eventually be replaced by a common kubernetes library.
func IsGranted(ctx context.Context, claim *v1.PersistentVolumeClaim, referenceGrants []*gatewayv1beta1.ReferenceGrant) (bool, error) {
	var allowed bool
	// Check that accessing to {namespace}/{name} is allowed.
	for _, grant := range referenceGrants {
		// The ReferenceGrant must be defined in the namespace of
		// the DataSourceRef.Namespace
		if grant.Namespace != *claim.Spec.DataSourceRef.Namespace {
			continue
		}

		var validFrom bool
		for _, from := range grant.Spec.From {
			if from.Group == "" && from.Kind == pvcKind && string(from.Namespace) == claim.Namespace {
				validFrom = true
				break
			}
		}
		// Skip unrelated policy by checking From field
		if !validFrom {
			continue
		}

		for _, to := range grant.Spec.To {
			if (claim.Spec.DataSourceRef.APIGroup != nil && string(to.Group) != *claim.Spec.DataSourceRef.APIGroup) ||
				(claim.Spec.DataSourceRef.APIGroup == nil && len(to.Group) > 0) ||
				string(to.Kind) != claim.Spec.DataSourceRef.Kind {
				continue
			}
			if to.Name == nil || string(*to.Name) == "" || string(*to.Name) == claim.Spec.DataSourceRef.Name {
				allowed = true
				break
			}
		}

		// If we got here, both the "from" and the "to" were allowed by this
		// reference grant.
		if allowed {
			return allowed, nil
		}
	}

	// If we got here, no reference policy or reference grant allowed both the "from" and "to".
	return false, fmt.Errorf("accessing %s/%s of %s dataSource from %s/%s isn't allowed", *claim.Spec.DataSourceRef.Namespace, claim.Spec.DataSourceRef.Name, claim.Spec.DataSourceRef.Kind, claim.Namespace, claim.Name)
}

func IsFinalError(err error) bool {
	// Sources:
	// https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
	// https://github.com/container-storage-interface/spec/blob/master/spec.md
	st, ok := status.FromError(err)
	if !ok {
		// This is not gRPC error. The operation must have failed before gRPC
		// method was called, otherwise we would get gRPC error.
		// We don't know if any previous volume operation is in progress, be on the safe side.
		return false
	}
	switch st.Code() {
	case codes.Canceled, // gRPC: Client Application cancelled the request
		codes.DeadlineExceeded,  // gRPC: Timeout
		codes.Unavailable,       // gRPC: Server shutting down, TCP connection broken - previous volume operation may be still in progress.
		codes.ResourceExhausted, // gRPC: Server temporarily out of resources - previous volume operation may be still in progress.
		codes.Aborted:           // CSI: Operation pending for volume
		return false
	}
	// All other errors mean that operation either did not
	// even start or failed. It is for sure not in progress.
	return true
}
