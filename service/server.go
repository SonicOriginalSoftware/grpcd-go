//revive:disable:package-comments
package service

import (
	"google.golang.org/grpc"

	"git.sonicoriginal.software/grpc-foundation/methods"
)

// Server is what this package needs of the caller's gRPC server: somewhere to
// register services, and a report of what ended up registered. Serving and
// stopping are the caller's, so they are not asked for here.
// *grpc.Server satisfies it; a test satisfies it with a recorder.
type Server interface {
	grpc.ServiceRegistrar
	methods.ServiceInfoProvider
}
