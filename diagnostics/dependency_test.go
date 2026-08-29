package diagnostics

import (
	"context"

	"git.sonicoriginal.software/grpc-testing/mocks/clientconn"

	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// dependencyStub supplies the connection details Dependency asks for. The
// shared mock only knows how to answer an RPC, and Go will not let this
// package attach methods to it, so the two are combined by embedding.
type dependencyStub struct {
	*clientconn.Mock

	address string
	state   connectivity.State
}

func (d *dependencyStub) Target() string { return d.address }

func (d *dependencyStub) GetState() connectivity.State { return d.state }

// respondServing answers the health check with SERVING.
func respondServing(_ context.Context, _ string, _, reply any) error {
	reply.(*grpc_health_v1.HealthCheckResponse).Status =
		grpc_health_v1.HealthCheckResponse_SERVING

	return nil
}

// newServingDependency is a ready connection that reports itself as serving.
func newServingDependency(address string) *dependencyStub {
	return &dependencyStub{
		Mock:    &clientconn.Mock{InvokeFn: respondServing},
		address: address,
		state:   connectivity.Ready,
	}
}

// newFailingDependency is a connection whose health check fails with err.
func newFailingDependency(address string, err error) *dependencyStub {
	return &dependencyStub{
		Mock:    &clientconn.Mock{Err: err},
		address: address,
		state:   connectivity.TransientFailure,
	}
}
