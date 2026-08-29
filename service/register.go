//revive:disable:package-comments
package service

import (
	"fmt"
	"os"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // Experimental gzip initialization
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"

	grpcdclient "git.sonicoriginal.software/grpcd-go/client"
	"git.sonicoriginal.software/grpcd-go/diagnostics"
)

// grpcdCheckName is the diagnostics name reserved for the grpcd dependency
const grpcdCheckName = "grpcd"

// Register attaches the caller's services to srv, along with the diagnostics,
// health, and reflection services every service exposes.
//
// A check for the grpcd dependency is added to checks, so that name is
// reserved and a caller supplying it is an error.
//
// Register only assembles. Serving, shutdown, and grpcd registration are the
// caller's to start.
func Register(
	srv Server,
	serviceName string,
	checks diagnostics.Checks,
	registerFn func(grpc.ServiceRegistrar),
) error {
	if srv == nil {
		return fmt.Errorf("srv cannot be nil")
	}

	if serviceName == "" {
		return fmt.Errorf("serviceName cannot be empty")
	}

	if checks == nil {
		checks = make(diagnostics.Checks)
	} else if _, exists := checks[grpcdCheckName]; exists {
		return fmt.Errorf("duplicate diagnostic name %q", grpcdCheckName)
	}

	grpcdAddress := os.Getenv(grpcdclient.GRPCDAddressKey)
	checks[grpcdCheckName] = diagnostics.NewTargetCheck(grpcdAddress)

	if registerFn != nil {
		registerFn(srv)
	}

	diagpb.RegisterDiagnosticsServiceServer(srv, diagnostics.NewServer(checks))

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthServer)
	healthServer.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(srv)

	return nil
}
