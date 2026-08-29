//revive:disable:package-comments
package service

import (
	"fmt"
	"os"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // Experimental gzip initialization
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"git.sonicoriginal.software/grpc-foundation/methods"
	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"

	grpcdclient "git.sonicoriginal.software/grpcd-go/client"
	"git.sonicoriginal.software/grpcd-go/diagnostics"
)

// grpcdCheckName is the diagnostics name reserved for the grpcd dependency
const grpcdCheckName = "grpcd"

// Register attaches the caller's services to srv, along with the diagnostics,
// health, and reflection services every service exposes. It marks each of the
// caller's services SERVING and returns their fully qualified method names,
// which is what grpcd advertises.
//
// A check for the grpcd dependency is added to checks, so that name is
// reserved and a caller supplying it is an error.
//
// Register only assembles. Serving, shutdown, grpcd registration, and any
// later health status change are the caller's to make.
func Register(
	srv Server,
	healthSrv Health,
	checks diagnostics.Checks,
	registerFn func(grpc.ServiceRegistrar),
) ([]string, error) {
	if srv == nil {
		return nil, fmt.Errorf("srv cannot be nil")
	}

	if healthSrv == nil {
		return nil, fmt.Errorf("healthSrv cannot be nil")
	}

	if checks == nil {
		checks = make(diagnostics.Checks)
	} else if _, exists := checks[grpcdCheckName]; exists {
		return nil, fmt.Errorf("duplicate diagnostic name %q", grpcdCheckName)
	}

	grpcdAddress := os.Getenv(grpcdclient.GRPCDAddressKey)
	checks[grpcdCheckName] = diagnostics.NewTargetCheck(grpcdAddress)

	if registerFn != nil {
		registerFn(srv)
	}

	diagpb.RegisterDiagnosticsServiceServer(srv, diagnostics.NewServer(checks))
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	reflection.Register(srv)

	methodNames := methods.Extract(srv, methods.NewPatternFilter(nil, infrastructurePrefixes))

	for _, serviceName := range serviceNames(methodNames) {
		healthSrv.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)
	}

	return methodNames, nil
}
