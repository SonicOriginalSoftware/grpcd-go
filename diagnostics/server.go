//revive:disable:package-comments
package diagnostics

import (
	"context"

	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"
)

// Server is a DiagnosticsService Server
type Server struct {
	diagpb.UnimplementedDiagnosticsServiceServer
	checks Checks
}

// NewServer returns a new DiagnosticsService Server
func NewServer(checks Checks) *Server {
	return &Server{checks: checks}
}

// GetDiagnostics collects diagnostics from the service's checks and reports them
func (s *Server) GetDiagnostics(
	ctx context.Context,
	_ *diagpb.GetDiagnosticsRequest,
) (response *diagpb.GetDiagnosticsResponse, err error) {
	c := newCollector(len(s.checks))
	for name, check := range s.checks {
		c.start(ctx, name, check)
	}

	return &diagpb.GetDiagnosticsResponse{
		Services: c.wait(),
	}, nil
}
