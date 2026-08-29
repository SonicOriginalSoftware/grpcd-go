//revive:disable:package-comments
package diagnostics

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"git.sonicoriginal.software/grpc-foundation/client"
	"git.sonicoriginal.software/grpc-foundation/health"
	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"
)

// Check inspects a dependency
type Check func(context.Context) (*diagpb.ServiceDependency, error)

// Checks is a bag of Checks
type Checks map[string]Check

func grpcCheck(ctx context.Context, conn Dependency) (*diagpb.ServiceDependency, error) {
	serving, err := health.Check(ctx, conn)

	return &diagpb.ServiceDependency{
		Address:     conn.Target(),
		Serving:     serving.String(),
		State:       conn.GetState().String(),
		LastChecked: time.Now().Unix(),
		Details:     map[string]string{},
	}, err
}

// NewDependencyCheck wires up checking an existing grpc connection
func NewDependencyCheck(conn Dependency) Check {
	return func(ctx context.Context) (*diagpb.ServiceDependency, error) {
		return grpcCheck(ctx, conn)
	}
}

// NewTargetCheck creating a new grpc connection before wiring up its checking
func NewTargetCheck(address string, opts ...grpc.DialOption) Check {
	return func(ctx context.Context) (*diagpb.ServiceDependency, error) {
		c, err := client.New(address, nil, nil, opts...)
		if err != nil {
			return nil, err
		}
		defer c.Close()

		return grpcCheck(ctx, c)
	}
}
