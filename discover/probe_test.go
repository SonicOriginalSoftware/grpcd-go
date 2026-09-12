package discover

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// connStub walks through states as reach asks about them. Each call to
// WaitForStateChange advances to the next state; running out ends the wait
// the way an expired context does.
type connStub struct {
	states []connectivity.State
	index  int
	closed bool
}

func (c *connStub) Connect() {}

func (c *connStub) GetState() connectivity.State { return c.states[c.index] }

func (c *connStub) WaitForStateChange(_ context.Context, _ connectivity.State) bool {
	if c.index+1 >= len(c.states) {
		return false
	}

	c.index++

	return true
}

func (c *connStub) Close() error {
	c.closed = true

	return nil
}

// refuseConnection stands in for the socket layer, so a probe can fail to
// connect without anything being dialled.
func refuseConnection(_ context.Context, _ string) (net.Conn, error) {
	return nil, errors.New("connection refused")
}

func TestReach(t *testing.T) {
	t.Run("answers once the connection is ready", func(t *testing.T) {
		c := &connStub{states: []connectivity.State{
			connectivity.Idle, connectivity.Connecting, connectivity.Ready,
		}}

		if err := reach(t.Context(), c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !c.closed {
			t.Error("expected the probe connection to be closed")
		}
	})

	t.Run("fails once the connection fails", func(t *testing.T) {
		c := &connStub{states: []connectivity.State{
			connectivity.Connecting, connectivity.TransientFailure,
		}}

		if err := reach(t.Context(), c); err == nil {
			t.Fatal("expected an error")
		}

		if !c.closed {
			t.Error("expected the probe connection to be closed")
		}
	})

	t.Run("fails once the connection shuts down", func(t *testing.T) {
		c := &connStub{states: []connectivity.State{connectivity.Shutdown}}

		if err := reach(t.Context(), c); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("gives up when the wait ends first", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		c := &connStub{states: []connectivity.State{connectivity.Connecting}}

		err := reach(ctx, c)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestNewProbe(t *testing.T) {
	t.Run("fails when the address cannot be parsed", func(t *testing.T) {
		// A control character fails URL parsing on both of grpc's attempts,
		// which is what makes building the connection itself fail.
		probe := NewProbe()

		if err := probe(t.Context(), "cache:443\n"); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("fails when the address does not answer", func(t *testing.T) {
		// The passthrough scheme hands the target straight to the dialer, so
		// the refusal is reached without a lookup.
		probe := NewProbe(grpc.WithContextDialer(refuseConnection))

		if err := probe(t.Context(), "passthrough:///queue:443"); err == nil {
			t.Fatal("expected an error")
		}
	})
}
