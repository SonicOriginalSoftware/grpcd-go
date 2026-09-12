package discover

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc/stats"
)

// remoteKey carries the peer address from TagConn to HandleConn, which is the
// only path grpc-go gives between the two.
type remoteKey struct{}

// view is what the sensor knows about the transports under one ClientConn:
// which address has a live transport right now, and which address most
// recently got one. It is replaced whole and never written to, so a reader
// sees one consistent pair.
type view struct {
	current   string
	lastBegan string
}

// sensor watches the transports under one ClientConn. grpc-go tells a resolver
// nothing when an established connection drops, so this is how the loop hears
// about it: HandleConn receives ConnBegin and ConnEnd as transports open and
// close.
//
// It also records which replica the connection is on, which is what
// diagnostics report; the ClientConn's own Target is the grpcd:/// URL.
type sensor struct {
	view atomic.Pointer[view]

	// changed says the view is worth a look. Buffered by one and sent to
	// without waiting, so HandleConn never blocks grpc-go and several changes
	// in a row raise it once: the loop reads the view, not the events.
	changed chan struct{}
}

func newSensor() *sensor {
	s := &sensor{changed: make(chan struct{}, 1)}
	s.view.Store(&view{})

	return s
}

// Address reports the replica currently connected, or "" when none is.
func (s *sensor) Address() string {
	return s.view.Load().current
}

// State reports what the sensor knows, for the loop to judge against the
// address it pushed.
func (s *sensor) State() view {
	return *s.view.Load()
}

// TagConn puts the peer address on the context HandleConn is later called with.
func (*sensor) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return context.WithValue(ctx, remoteKey{}, info.RemoteAddr.String())
}

// HandleConn records a transport opening or closing. ConnBegin and ConnEnd
// are the only kinds grpc-go sends, and nothing outside it can add one.
func (s *sensor) HandleConn(ctx context.Context, event stats.ConnStats) {
	address, _ := ctx.Value(remoteKey{}).(string)

	if _, begun := event.(*stats.ConnBegin); begun {
		s.view.Store(&view{current: address, lastBegan: address})
	} else {
		s.end(address)
	}

	select {
	case s.changed <- struct{}{}:
	default:
	}
}

// end forgets address as current, if it is. Transports open and close on
// their own goroutines, so the replacement is retried until it lands on the
// view it was computed from.
func (s *sensor) end(address string) {
	for {
		before := s.view.Load()

		if before.current != address {
			return
		}

		after := &view{lastBegan: before.lastBegan}

		if s.view.CompareAndSwap(before, after) {
			return
		}
	}
}

// TagRPC is required by stats.Handler and unused here.
func (*sensor) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context { return ctx }

// HandleRPC is required by stats.Handler and unused here.
func (*sensor) HandleRPC(context.Context, stats.RPCStats) {}
