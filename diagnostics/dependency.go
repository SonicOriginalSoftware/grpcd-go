//revive:disable:package-comments
package diagnostics

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Dependency holds information about a client's connection to a downstream gRPC service
type Dependency interface {
	grpc.ClientConnInterface
	Target() string
	GetState() connectivity.State
}

// Dependencies is a bag of Dependency, keyed by a name
type Dependencies map[string]Dependency
