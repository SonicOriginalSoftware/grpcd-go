package service

import (
	"context"
	"os"

	"git.sonicoriginal.software/grpc-protos/info"
)

// GetInfo returns service metadata (implements lib.InfoServiceServer)
func (s *Server) GetInfo(
	_ context.Context, _ *info.GetInfoRequest,
) (*info.GetInfoResponse, error) {
	version := os.Getenv("SERVICE_VERSION")

	return &info.GetInfoResponse{
		Info: &info.ServiceInformation{
			Version: version,
			Details: make(map[string]string),
		},
	}, nil
}
