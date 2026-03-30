package service

import (
	"context"
	"os"
	"time"

	"git.sonicoriginal.software/grpcd-go/client"

	"git.sonicoriginal.software/grpc-foundation/health"
	"git.sonicoriginal.software/grpc-protos/diagnostics"
)

// GetDiagnostics returns diagnostic information about service dependencies (implements lib.DiagnosticsServiceServer)
func (s *Server) GetDiagnostics(
	ctx context.Context, _ *diagnostics.GetDiagnosticsRequest,
) (*diagnostics.GetDiagnosticsResponse, error) {
	services := make(map[string]*diagnostics.ServiceDependency)

	// Check grpcd service reachability
	grpcdAddr := os.Getenv(client.GRPCDAddressKey)
	h := health.Check(ctx, grpcdAddr)
	grpcdCheck := &diagnostics.ServiceDependency{
		Address:     grpcdAddr,
		State:       h,
		Serving:     "",
		LastChecked: time.Now().Unix(),
		Details:     map[string]string{},
	}
	services["grpcd"] = grpcdCheck

	return &diagnostics.GetDiagnosticsResponse{Services: services}, nil
}
