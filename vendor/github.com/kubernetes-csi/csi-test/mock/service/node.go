package service

import (
	"path"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"

	"github.com/container-storage-interface/spec/lib/go/csi/v0"
)

func (s *service) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse, error) {

	device, ok := req.PublishInfo["device"]
	if !ok {
		return nil, status.Error(
			codes.InvalidArgument,
			"stage volume info 'device' key required")
	}

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}

	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path cannot be empty")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capability cannot be empty")
	}

	s.volsRWL.Lock()
	defer s.volsRWL.Unlock()

	i, v := s.findVolNoLock("id", req.VolumeId)
	if i < 0 {
		return nil, status.Error(codes.NotFound, req.VolumeId)
	}

	// nodeStgPathKey is the key in the volume's attributes that is set to a
	// mock stage path if the volume has been published by the node
	nodeStgPathKey := path.Join(s.nodeID, req.StagingTargetPath)

	// Check to see if the volume has already been staged.
	if v.Attributes[nodeStgPathKey] != "" {
		// TODO: Check for the capabilities to be equal. Return "ALREADY_EXISTS"
		// if the capabilities don't match.
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// Stage the volume.
	v.Attributes[nodeStgPathKey] = device
	s.vols[i] = v

	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *service) NodeUnstageVolume(
	ctx context.Context,
	req *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse, error) {

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}

	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path cannot be empty")
	}

	s.volsRWL.Lock()
	defer s.volsRWL.Unlock()

	i, v := s.findVolNoLock("id", req.VolumeId)
	if i < 0 {
		return nil, status.Error(codes.NotFound, req.VolumeId)
	}

	// nodeStgPathKey is the key in the volume's attributes that is set to a
	// mock stage path if the volume has been published by the node
	nodeStgPathKey := path.Join(s.nodeID, req.StagingTargetPath)

	// Check to see if the volume has already been unstaged.
	if v.Attributes[nodeStgPathKey] == "" {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	// Unpublish the volume.
	delete(v.Attributes, nodeStgPathKey)
	s.vols[i] = v

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse, error) {

	device, ok := req.PublishInfo["device"]
	if !ok {
		return nil, status.Error(
			codes.InvalidArgument,
			"publish volume info 'device' key required")
	}

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}

	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path cannot be empty")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capability cannot be empty")
	}

	s.volsRWL.Lock()
	defer s.volsRWL.Unlock()

	i, v := s.findVolNoLock("id", req.VolumeId)
	if i < 0 {
		return nil, status.Error(codes.NotFound, req.VolumeId)
	}

	// nodeMntPathKey is the key in the volume's attributes that is set to a
	// mock mount path if the volume has been published by the node
	nodeMntPathKey := path.Join(s.nodeID, req.TargetPath)

	// Check to see if the volume has already been published.
	if v.Attributes[nodeMntPathKey] != "" {

		// Requests marked Readonly fail due to volumes published by
		// the Mock driver supporting only RW mode.
		if req.Readonly {
			return nil, status.Error(codes.AlreadyExists, req.VolumeId)
		}

		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Publish the volume.
	if req.GetStagingTargetPath() != "" {
		v.Attributes[nodeMntPathKey] = req.GetStagingTargetPath()
	} else {
		v.Attributes[nodeMntPathKey] = device
	}
	s.vols[i] = v

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *service) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse, error) {

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path cannot be empty")
	}

	s.volsRWL.Lock()
	defer s.volsRWL.Unlock()

	i, v := s.findVolNoLock("id", req.VolumeId)
	if i < 0 {
		return nil, status.Error(codes.NotFound, req.VolumeId)
	}

	// nodeMntPathKey is the key in the volume's attributes that is set to a
	// mock mount path if the volume has been published by the node
	nodeMntPathKey := path.Join(s.nodeID, req.TargetPath)

	// Check to see if the volume has already been unpublished.
	if v.Attributes[nodeMntPathKey] == "" {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	// Unpublish the volume.
	delete(v.Attributes, nodeMntPathKey)
	s.vols[i] = v

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *service) NodeGetId(
	ctx context.Context,
	req *csi.NodeGetIdRequest) (
	*csi.NodeGetIdResponse, error) {

	return &csi.NodeGetIdResponse{
		NodeId: s.nodeID,
	}, nil
}

func (s *service) NodeGetCapabilities(
	ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse, error) {

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (s *service) NodeGetInfo(ctx context.Context,
	req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeID,
	}, nil
}
