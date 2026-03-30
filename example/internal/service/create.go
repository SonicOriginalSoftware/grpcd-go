package service

import (
	"context"

	"git.sonicoriginal.software/grpcd-go/example/internal/station"
)

// Create creates an example
func (s *Server) Create(
	ctx context.Context, _ *station.CreateRequest,
) (*station.CreateResponse, error) {
	if s.createCount != nil {
		s.createCount.Add(ctx, 1)
	}

	return &station.CreateResponse{}, nil
}
