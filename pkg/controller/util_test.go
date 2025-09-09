package controller

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func ObjectNamePtr(val string) *gatewayv1beta1.ObjectName {
	objectName := gatewayv1beta1.ObjectName(val)
	return &objectName
}

// generateReferenceGrant returns a new ReferenceGrant with given attributes
func generateReferenceGrant(namespace string, referenceGrantFrom []gatewayv1beta1.ReferenceGrantFrom, referenceGrantTo []gatewayv1beta1.ReferenceGrantTo) *gatewayv1beta1.ReferenceGrant {
	return &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "refGrant",
			Namespace: namespace,
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: referenceGrantFrom,
			To:   referenceGrantTo,
		},
	}
}

func generatePVCForFromXnsdataSource(namespace string, dataSourceRef *v1.TypedObjectReference, requestedBytes int64) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pvc",
			Namespace:   namespace,
			UID:         "testid",
			Annotations: driverNameAnnotation,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			Selector: nil,
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): *resource.NewQuantity(requestedBytes, resource.BinarySI),
				},
			},
			AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			DataSourceRef: dataSourceRef,
		},
	}
}

func TestIsGranted(t *testing.T) {
	ctx := context.Background()
	var requestedBytes int64 = 1000
	coreapiGrp := ""
	pvcapiKind := "PersistentVolumeClaim"
	dataSourceName := "test-dataSource"
	dataSourceNamespace := "ns1"
	xnsNamespace := "ns2"
	snapapiGrp := "snapshot.storage.k8s.io"
	snapKind := "VolumeSnapshot"
	anyvolumedatasourceapiGrp := "hello.k8s.io/v1alpha1"
	anyvolumedatasourceapiKind := "Hello"

	type testcase struct {
		name                   string
		expectErr              bool
		pvcNamespace           string
		dataSourceRef          *v1.TypedObjectReference
		refGrantsrcNamespace   string
		referenceGrantFrom     []gatewayv1beta1.ReferenceGrantFrom
		referenceGrantTo       []gatewayv1beta1.ReferenceGrantTo
		withoutreferenceGrants bool
	}

	var testcases []*testcase
	baseCase := func() *testcase {
		return &testcase{
			name:         "Allowed to access dataSource for xns PVC with refGrant",
			expectErr:    false,
			pvcNamespace: xnsNamespace,
			dataSourceRef: &v1.TypedObjectReference{
				APIGroup:  &coreapiGrp,
				Kind:      pvcapiKind,
				Name:      dataSourceName,
				Namespace: &dataSourceNamespace,
			},
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind(pvcapiKind),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind(pvcapiKind),
				},
			},
			withoutreferenceGrants: false,
		}
	}
	testcases = append(testcases, baseCase())

	modified := baseCase()
	modified.name = "Allowed to access dataSource for xns PVC with refGrant of specify toName"
	modified.referenceGrantTo[0].Name = ObjectNamePtr(dataSourceName)
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Allowed to access dataSource of apiGroup nil for xns PVC with refGrant"
	modified.dataSourceRef.APIGroup = nil
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Allowed to access dataSource for xns PVC with refGrant of specify toName of nil"
	modified.referenceGrantTo[0].Name = nil
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Allowed to access dataSource for xns PVC with refGrant of specify toName of non"
	modified.referenceGrantTo[0].Name = ObjectNamePtr("")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Allowed to access dataSource for xns Snapshot with refGrant"
	modified.dataSourceRef.APIGroup = &snapapiGrp
	modified.dataSourceRef.Kind = snapKind
	modified.referenceGrantTo[0].Group = gatewayv1beta1.Group(snapapiGrp)
	modified.referenceGrantTo[0].Kind = gatewayv1beta1.Kind(snapKind)
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Allowed to access dataSource for xns AnyVolumeDataSource with refGrant"
	modified.dataSourceRef.APIGroup = &anyvolumedatasourceapiGrp
	modified.dataSourceRef.Kind = anyvolumedatasourceapiKind
	modified.referenceGrantTo[0].Group = gatewayv1beta1.Group(anyvolumedatasourceapiGrp)
	modified.referenceGrantTo[0].Kind = gatewayv1beta1.Kind(anyvolumedatasourceapiKind)
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC without refGrant"
	modified.expectErr = true
	modified.withoutreferenceGrants = true
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns Snapshot without refGrant"
	modified.expectErr = true
	modified.dataSourceRef.APIGroup = &snapapiGrp
	modified.dataSourceRef.Kind = snapKind
	modified.withoutreferenceGrants = true
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns AnyVolumeDataSource without refGrant"
	modified.expectErr = true
	modified.dataSourceRef.APIGroup = &anyvolumedatasourceapiGrp
	modified.dataSourceRef.Kind = anyvolumedatasourceapiKind
	modified.withoutreferenceGrants = true
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong create namespace"
	modified.expectErr = true
	modified.refGrantsrcNamespace = "wrong-reference-namespace"
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong fromGroup"
	modified.expectErr = true
	modified.referenceGrantFrom[0].Group = gatewayv1beta1.Group("wrong.fromGroup")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong fromKind"
	modified.expectErr = true
	modified.referenceGrantFrom[0].Group = gatewayv1beta1.Group("wrongfromKind")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong fromNamespace"
	modified.expectErr = true
	modified.referenceGrantFrom[0].Namespace = gatewayv1beta1.Namespace("wrong-fromNamespace")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong toGroup"
	modified.expectErr = true
	modified.referenceGrantTo[0].Group = gatewayv1beta1.Group("wrong.toGroup")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong toKind"
	modified.expectErr = true
	modified.referenceGrantTo[0].Kind = gatewayv1beta1.Kind("WrongtoKind")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of wrong toName"
	modified.expectErr = true
	modified.referenceGrantTo[0].Name = ObjectNamePtr("wrong-dataSource-name")
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access PVC dataSource for xns PVC with refGrant of SnapShot dataSource"
	modified.expectErr = true
	modified.referenceGrantTo[0].Group = gatewayv1beta1.Group(snapapiGrp)
	modified.referenceGrantTo[0].Kind = gatewayv1beta1.Kind(snapKind)
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access Snapshot dataSource of apiGroup nil for xns Snapshot with refGrant"
	modified.expectErr = true
	modified.dataSourceRef.APIGroup = nil
	modified.dataSourceRef.Kind = snapKind
	modified.referenceGrantTo[0].Group = gatewayv1beta1.Group(snapapiGrp)
	modified.referenceGrantTo[0].Kind = gatewayv1beta1.Kind(snapKind)
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access Snapshot dataSource for xns PVC with refGrant of PVC dataSource"
	modified.expectErr = true
	modified.dataSourceRef.APIGroup = &snapapiGrp
	modified.dataSourceRef.Kind = snapKind
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of referenceGrantFrom empty"
	modified.expectErr = true
	modified.referenceGrantFrom = []gatewayv1beta1.ReferenceGrantFrom{}
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of referenceGrantTo empty"
	modified.expectErr = true
	modified.referenceGrantTo = []gatewayv1beta1.ReferenceGrantTo{}
	testcases = append(testcases, modified)

	modified = baseCase()
	modified.name = "Not allowed to access dataSource for xns PVC with refGrant of referenceGrantFrom and referenceGrantTo empty"
	modified.expectErr = true
	modified.referenceGrantFrom = []gatewayv1beta1.ReferenceGrantFrom{}
	modified.referenceGrantTo = []gatewayv1beta1.ReferenceGrantTo{}
	testcases = append(testcases, modified)

	doit := func(t *testing.T, tc testcase) {
		var referenceGrantList []*gatewayv1beta1.ReferenceGrant
		if tc.withoutreferenceGrants {
			referenceGrantList = nil
		} else {
			referenceGrant := generateReferenceGrant(tc.refGrantsrcNamespace, tc.referenceGrantFrom, tc.referenceGrantTo)
			referenceGrantList = append(referenceGrantList, referenceGrant)
		}
		claim := generatePVCForFromXnsdataSource(tc.pvcNamespace, tc.dataSourceRef, requestedBytes)
		allowed, err := IsGranted(ctx, claim, referenceGrantList)
		if tc.expectErr && (err == nil || allowed) {
			t.Errorf("Expected error, got none")
		}
		if !tc.expectErr && (err != nil || !allowed) {
			t.Errorf("got error: %v", err)
		}
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			doit(t, *tc)
		})
	}
}
