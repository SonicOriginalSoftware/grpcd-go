package discover

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	grpcd "github.com/grpcd/protos"
)

// loop is the resolver grpc-go holds for one ClientConn. It discovers an
// address, pushes it, holds it until the transport to it drops or grpcd says
// to move, and discovers again, for as long as the connection exists.
type loop struct {
	upstream *Upstream
	cc       resolver.ClientConn
	cancel   context.CancelFunc
}

// ResolveNow is grpc-go's hint to resolve again. The loop already rediscovers
// on the one event that matters, the transport dropping, so the hint is
// ignored as its contract allows.
func (*loop) ResolveNow(resolver.ResolveNowOptions) {}

// Close stops the loop. grpc-go calls it when the ClientConn is closed.
func (l *loop) Close() {
	l.cancel()
}

// run is the loop. Before each discovery the connection is given no addresses,
// so an RPC made while nothing is held fails at once rather than waiting on a
// dial to an address that just died.
func (l *loop) run(ctx context.Context) {
	for ctx.Err() == nil {
		// The balancer answers an empty list with an error, which is grpc-go
		// saying it will not retry; there is nothing to retry, so it is ignored.
		_ = l.cc.UpdateState(resolver.State{})

		address, w, found := l.discover(ctx)
		if !found {
			return
		}

		l.push(address)

		w = l.hold(ctx, address, w)

		w.stop()
	}
}

// push gives the connection one address.
func (l *loop) push(address string) {
	_ = l.cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: address}}})
}

// discover asks until a candidate probes reachable, answering with it and the
// watcher holding a Watch on it. It answers false only when ctx ends. A stream
// that ends without an answer is reopened; the wait for grpcd itself is the
// connection's own backoff, reached through WaitForReady.
func (l *loop) discover(ctx context.Context) (string, *watcher, bool) {
	for ctx.Err() == nil {
		if address, w, found := l.ask(ctx); found {
			return address, w, true
		}
	}

	return "", nil, false
}

// ask works one Discover stream: it takes the first candidate that probes
// reachable, reports each that does not, and answers false when the stream
// ends first.
//
// A Watch on the taken address is open before the Discover stream is closed,
// so no registration falls between the two. Closing the stream is how grpcd
// is told the candidate worked, and the stream's context is cancelled on the
// way out so it ends whether or not that close is delivered.
func (l *loop) ask(ctx context.Context) (string, *watcher, bool) {
	ctx, span := l.upstream.discovery.tracer.Start(ctx, "discover")
	defer span.End()

	askCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	log := l.upstream.log

	stream, err := l.upstream.discovery.service.Discover(askCtx, grpc.WaitForReady(true))
	if err != nil {
		log.ErrorContext(ctx, "Failed to open discovery", slog.Any("error", err))

		return "", nil, false
	}

	request := &grpcd.DiscoverRequest{
		Step: &grpcd.DiscoverRequest_MethodName{MethodName: l.upstream.method},
	}

	if err = stream.Send(request); err != nil {
		log.ErrorContext(ctx, "Failed to ask for the method", slog.Any("error", err))

		return "", nil, false
	}

	for {
		response, err := stream.Recv()
		if err != nil {
			log.ErrorContext(ctx, "Discovery ended without an address", slog.Any("error", err))

			return "", nil, false
		}

		address := response.GetAddress()

		if err = l.upstream.discovery.probe(ctx, address); err != nil {
			log.InfoContext(ctx, "Candidate unreachable, reporting it dead",
				slog.String("address", address), slog.Any("error", err))

			dead := &grpcd.DiscoverRequest{
				Step: &grpcd.DiscoverRequest_DeadAddress{DeadAddress: address},
			}

			if err = stream.Send(dead); err != nil {
				log.ErrorContext(ctx, "Failed to report the candidate dead", slog.Any("error", err))

				return "", nil, false
			}

			continue
		}

		// The watcher lives under the loop's context, not this ask's, so it
		// outlives the stream that found the address and stops with the loop.
		w := l.watch(ctx, address)

		if !l.opened(ctx, w) {
			w.stop()

			return "", nil, false
		}

		// Delivery of the close is not waited on: the address is held either
		// way, and cancel above ends the stream regardless.
		_ = stream.CloseSend()

		log.InfoContext(ctx, "Upstream discovered", slog.String("address", address))

		return address, w, true
	}
}

// opened waits for w's first Watch to be open, answering false if ctx ends
// first.
func (*loop) opened(ctx context.Context, w *watcher) bool {
	select {
	case <-w.opened:
		return true
	case <-ctx.Done():
		return false
	}
}

// hold keeps address until the transport to it drops or ctx ends, moving to
// whatever grpcd says to move to along the way. It answers with the watcher
// on whatever address it ends holding, for the caller to stop.
func (l *loop) hold(ctx context.Context, address string, w *watcher) *watcher {
	log := l.upstream.log

	for {
		select {
		case <-ctx.Done():
			return w

		case <-l.upstream.sensor.changed:
			if !l.dropped(address) {
				continue
			}

			log.InfoContext(ctx, "Upstream connection dropped", slog.String("address", address))

			return w

		case next, open := <-w.moves:
			if !open {
				// The watcher stops only with its context, which is the loop's.
				return w
			}

			if err := l.upstream.discovery.probe(ctx, next); err != nil {
				log.InfoContext(ctx, "Told to move to an unreachable address, staying",
					slog.String("address", next), slog.Any("error", err))

				continue
			}

			// The new Watch is open before the old one is closed, so no
			// registration falls between them.
			nw := l.watch(ctx, next)

			if !l.opened(ctx, nw) {
				nw.stop()

				return w
			}

			l.push(next)
			w.stop()

			log.InfoContext(ctx, "Moved", slog.String("from", address), slog.String("to", next))

			address, w = next, nw
		}
	}
}

// dropped judges the sensor's view against the address the loop pushed. A
// live transport to it means nothing is wrong; a transport it once had and
// no longer has means it dropped; anything else means it has not connected
// yet, which includes an older transport ending on its own time.
func (l *loop) dropped(address string) bool {
	state := l.upstream.sensor.State()

	if state.current == address {
		return false
	}

	return state.lastBegan == address
}
