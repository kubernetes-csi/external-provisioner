// +build !ignore_autogenerated

/*
Copyright 2018 The Kubernetes Authors.

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

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSElasticBlockStoreVolumeSnapshotSource) DeepCopyInto(out *AWSElasticBlockStoreVolumeSnapshotSource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSElasticBlockStoreVolumeSnapshotSource.
func (in *AWSElasticBlockStoreVolumeSnapshotSource) DeepCopy() *AWSElasticBlockStoreVolumeSnapshotSource {
	if in == nil {
		return nil
	}
	out := new(AWSElasticBlockStoreVolumeSnapshotSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CinderVolumeSnapshotSource) DeepCopyInto(out *CinderVolumeSnapshotSource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CinderVolumeSnapshotSource.
func (in *CinderVolumeSnapshotSource) DeepCopy() *CinderVolumeSnapshotSource {
	if in == nil {
		return nil
	}
	out := new(CinderVolumeSnapshotSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GCEPersistentDiskSnapshotSource) DeepCopyInto(out *GCEPersistentDiskSnapshotSource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GCEPersistentDiskSnapshotSource.
func (in *GCEPersistentDiskSnapshotSource) DeepCopy() *GCEPersistentDiskSnapshotSource {
	if in == nil {
		return nil
	}
	out := new(GCEPersistentDiskSnapshotSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlusterVolumeSnapshotSource) DeepCopyInto(out *GlusterVolumeSnapshotSource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlusterVolumeSnapshotSource.
func (in *GlusterVolumeSnapshotSource) DeepCopy() *GlusterVolumeSnapshotSource {
	if in == nil {
		return nil
	}
	out := new(GlusterVolumeSnapshotSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostPathVolumeSnapshotSource) DeepCopyInto(out *HostPathVolumeSnapshotSource) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostPathVolumeSnapshotSource.
func (in *HostPathVolumeSnapshotSource) DeepCopy() *HostPathVolumeSnapshotSource {
	if in == nil {
		return nil
	}
	out := new(HostPathVolumeSnapshotSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshot) DeepCopyInto(out *VolumeSnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Metadata.DeepCopyInto(&out.Metadata)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshot.
func (in *VolumeSnapshot) DeepCopy() *VolumeSnapshot {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshot) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotCondition.
func (in *VolumeSnapshotCondition) DeepCopy() *VolumeSnapshotCondition {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotCopy) DeepCopyInto(out *VolumeSnapshotCopy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Metadata.DeepCopyInto(&out.Metadata)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotCopy.
func (in *VolumeSnapshotCopy) DeepCopy() *VolumeSnapshotCopy {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotCopy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotData) DeepCopyInto(out *VolumeSnapshotData) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Metadata.DeepCopyInto(&out.Metadata)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotData.
func (in *VolumeSnapshotData) DeepCopy() *VolumeSnapshotData {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotData)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotData) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataCondition.
func (in *VolumeSnapshotDataCondition) DeepCopy() *VolumeSnapshotDataCondition {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotDataCopy) DeepCopyInto(out *VolumeSnapshotDataCopy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Metadata.DeepCopyInto(&out.Metadata)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataCopy.
func (in *VolumeSnapshotDataCopy) DeepCopy() *VolumeSnapshotDataCopy {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataCopy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotDataList) DeepCopyInto(out *VolumeSnapshotDataList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.Metadata = in.Metadata
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSnapshotData, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataList.
func (in *VolumeSnapshotDataList) DeepCopy() *VolumeSnapshotDataList {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotDataList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotDataListCopy) DeepCopyInto(out *VolumeSnapshotDataListCopy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.Metadata = in.Metadata
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSnapshotData, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataListCopy.
func (in *VolumeSnapshotDataListCopy) DeepCopy() *VolumeSnapshotDataListCopy {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataListCopy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotDataSource) DeepCopyInto(out *VolumeSnapshotDataSource) {
	*out = *in
	if in.HostPath != nil {
		in, out := &in.HostPath, &out.HostPath
		if *in == nil {
			*out = nil
		} else {
			*out = new(HostPathVolumeSnapshotSource)
			**out = **in
		}
	}
	if in.GlusterSnapshotVolume != nil {
		in, out := &in.GlusterSnapshotVolume, &out.GlusterSnapshotVolume
		if *in == nil {
			*out = nil
		} else {
			*out = new(GlusterVolumeSnapshotSource)
			**out = **in
		}
	}
	if in.AWSElasticBlockStore != nil {
		in, out := &in.AWSElasticBlockStore, &out.AWSElasticBlockStore
		if *in == nil {
			*out = nil
		} else {
			*out = new(AWSElasticBlockStoreVolumeSnapshotSource)
			**out = **in
		}
	}
	if in.GCEPersistentDiskSnapshot != nil {
		in, out := &in.GCEPersistentDiskSnapshot, &out.GCEPersistentDiskSnapshot
		if *in == nil {
			*out = nil
		} else {
			*out = new(GCEPersistentDiskSnapshotSource)
			**out = **in
		}
	}
	if in.CinderSnapshot != nil {
		in, out := &in.CinderSnapshot, &out.CinderSnapshot
		if *in == nil {
			*out = nil
		} else {
			*out = new(CinderVolumeSnapshotSource)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataSource.
func (in *VolumeSnapshotDataSource) DeepCopy() *VolumeSnapshotDataSource {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataSpec.
func (in *VolumeSnapshotDataSpec) DeepCopy() *VolumeSnapshotDataSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotDataStatus) DeepCopyInto(out *VolumeSnapshotDataStatus) {
	*out = *in
	in.CreationTimestamp.DeepCopyInto(&out.CreationTimestamp)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]VolumeSnapshotDataCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotDataStatus.
func (in *VolumeSnapshotDataStatus) DeepCopy() *VolumeSnapshotDataStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotDataStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotList) DeepCopyInto(out *VolumeSnapshotList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.Metadata = in.Metadata
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSnapshot, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotList.
func (in *VolumeSnapshotList) DeepCopy() *VolumeSnapshotList {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotListCopy) DeepCopyInto(out *VolumeSnapshotListCopy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.Metadata = in.Metadata
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSnapshot, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotListCopy.
func (in *VolumeSnapshotListCopy) DeepCopy() *VolumeSnapshotListCopy {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotListCopy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotSpec) DeepCopyInto(out *VolumeSnapshotSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotSpec.
func (in *VolumeSnapshotSpec) DeepCopy() *VolumeSnapshotSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotStatus) DeepCopyInto(out *VolumeSnapshotStatus) {
	*out = *in
	in.CreationTimestamp.DeepCopyInto(&out.CreationTimestamp)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]VolumeSnapshotCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotStatus.
func (in *VolumeSnapshotStatus) DeepCopy() *VolumeSnapshotStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotStatus)
	in.DeepCopyInto(out)
	return out
}
