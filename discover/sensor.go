package discover

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc/stats"
)

// remoteKey carries the peer address from TagConn to HandleConn, which is the
// only path grpc-go gives between the two.
type remoteKey struct{}

// sensor watches the transports under one ClientConn. grpc-go tells a resolver
// nothing when an established connection drops, so this is how the loop hears
// about it: HandleConn receives ConnEnd when a transport closes.
//
// It also records which replica the connection is on, which is what
// diagnostics report; the ClientConn's own Target is the grpcd:/// URL.
type sensor struct {
	address atomic.Pointer[string]

	// dropped carries the address of each transport that closed. Buffered by
	// one so HandleConn never blocks grpc-go, and sent to without waiting so a
	// second close while the loop is busy is dropped rather than queued: the
	// loop rediscovers once either way.
	dropped chan string
}

func newSensor() *sensor {
	s := &sensor{dropped: make(chan string, 1)}
	s.address.Store(new(string))

	return s
}

// Address reports the replica currently connected, or "" when none is.
func (s *sensor) Address() string {
	return *s.address.Load()
}

// TagConn puts the peer address on the context HandleConn is later called with.
func (*sensor) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return context.WithValue(ctx, remoteKey{}, info.RemoteAddr.String())
}

// HandleConn records a transport opening and reports one closing.
func (s *sensor) HandleConn(ctx context.Context, event stats.ConnStats) {
	address, _ := ctx.Value(remoteKey{}).(string)

	switch event.(type) {
	case *stats.ConnBegin:
		s.address.Store(&address)
	case *stats.ConnEnd:
		s.address.Store(new(string))

		select {
		case s.dropped <- address:
		default:
		}
	}
}

// TagRPC is required by stats.Handler and unused here.
func (*sensor) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context { return ctx }

// HandleRPC is required by stats.Handler and unused here.
func (*sensor) HandleRPC(context.Context, stats.RPCStats) {}
