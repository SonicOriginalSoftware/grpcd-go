//revive:disable:package-comments
package service

import (
	"git.sonicoriginal.software/grpc-foundation/methods"
)

// infrastructurePrefixes name the services a caller never advertises to grpcd
var infrastructurePrefixes = []string{
	"grpc.",        // gRPC infrastructure (health, reflection)
	"info.",        // Cumulus internal info endpoint
	"diagnostics.", // Cumulus internal diagnostics endpoint
}

// Methods returns the fully qualified method names to advertise to grpcd,
// excluding infrastructure services. Call it after Register, since it reports
// what is registered on srv at the moment it is called.
func Methods(srv Server) []string {
	filter := methods.NewPatternFilter(nil, infrastructurePrefixes)

	return methods.Extract(srv, filter)
}
