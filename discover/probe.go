package discover

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	foundationclient "git.sonicoriginal.software/grpc-foundation/client"
)

// Probe reports whether address can be reached. It is the client's own dial,
// from the client's own network position: grpcd never dials anything, and an
// address is reported dead only because a probe of it failed.
type Probe func(ctx context.Context, address string) error

// conn is what reach needs of a connection: the four methods it drives.
// *grpc.ClientConn satisfies it; a test satisfies it with a fake.
type conn interface {
	Connect()
	GetState() connectivity.State
	WaitForStateChange(ctx context.Context, source connectivity.State) bool
	Close() error
}

// NewProbe answers with the Probe production uses: dial through
// foundationclient.New, wait for the connection to become Ready or fail, and
// close it either way. opts are appended to the standard ones, which is how a
// test substitutes the dialer.
func NewProbe(opts ...grpc.DialOption) Probe {
	return func(ctx context.Context, address string) error {
		c, err := foundationclient.New(address, nil, nil, opts...)
		if err != nil {
			return err
		}

		return reach(ctx, c)
	}
}

// reach connects c and waits for a verdict: nil once it is Ready, an error once
// it fails or ctx ends. The connection is closed on the way out either way,
// because a probe only asks the question.
func reach(ctx context.Context, c conn) error {
	defer c.Close()

	c.Connect()

	for {
		state := c.GetState()

		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.TransientFailure, connectivity.Shutdown:
			return fmt.Errorf("connection entered %s", state)
		}

		if !c.WaitForStateChange(ctx, state) {
			return errors.Join(ctx.Err(), errors.New("gave up waiting for the connection"))
		}
	}
}
