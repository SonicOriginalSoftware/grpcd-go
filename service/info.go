//revive:disable:package-comments
package service

import (
	"context"

	infopb "git.sonicoriginal.software/grpc-protos/info"
)

// infoServer reports facts about the running server. Every service exposes the
// same answers, so this is assembled rather than reimplemented per service.
type infoServer struct {
	infopb.UnimplementedInfoServiceServer

	version string
}

// Version reports the running server's version.
func (s *infoServer) Version(
	_ context.Context, _ *infopb.VersionRequest,
) (*infopb.VersionResponse, error) {
	return &infopb.VersionResponse{Version: s.version}, nil
}
