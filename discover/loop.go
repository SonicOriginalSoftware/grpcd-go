package discover

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"

	grpcd "git.sonicoriginal.software/grpcd-protos"
)

// loop is the resolver grpc-go holds for one ClientConn. It discovers an
// address, pushes it, waits for the transport to it to drop, and discovers
// again, for as long as the connection exists.
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

		address, found := l.discover(ctx)
		if !found {
			return
		}

		_ = l.cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: address}}})

		l.await(ctx, address)
	}
}

// discover asks until a candidate probes reachable. It answers false only
// when ctx ends. A stream that ends without an answer is reopened; the wait
// for grpcd itself is the connection's own backoff, reached through
// WaitForReady.
func (l *loop) discover(ctx context.Context) (string, bool) {
	for ctx.Err() == nil {
		if address, found := l.ask(ctx); found {
			return address, true
		}
	}

	return "", false
}

// ask works one Discover stream: it takes the first candidate that probes
// reachable, reports each that does not, and answers false when the stream
// ends first. Closing the stream is how grpcd is told the last candidate
// worked, and the stream's context is cancelled on the way out so it ends
// whether or not that close is delivered.
func (l *loop) ask(ctx context.Context) (string, bool) {
	ctx, span := l.upstream.discovery.tracer.Start(ctx, "discover")
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log := l.upstream.log

	stream, err := l.upstream.discovery.service.Discover(ctx, grpc.WaitForReady(true))
	if err != nil {
		log.ErrorContext(ctx, "Failed to open discovery", slog.Any("error", err))

		return "", false
	}

	request := &grpcd.DiscoverRequest{
		Step: &grpcd.DiscoverRequest_MethodName{MethodName: l.upstream.method},
	}

	if err = stream.Send(request); err != nil {
		log.ErrorContext(ctx, "Failed to ask for the method", slog.Any("error", err))

		return "", false
	}

	for {
		response, err := stream.Recv()
		if err != nil {
			log.ErrorContext(ctx, "Discovery ended without an address", slog.Any("error", err))

			return "", false
		}

		address := response.GetAddress()

		if err = l.upstream.discovery.probe(ctx, address); err == nil {
			// Delivery of the close is not waited on: the address is held
			// either way, and cancel above ends the stream regardless.
			_ = stream.CloseSend()

			log.InfoContext(ctx, "Upstream discovered", slog.String("address", address))

			return address, true
		}

		log.InfoContext(ctx, "Candidate unreachable, reporting it dead",
			slog.String("address", address), slog.Any("error", err))

		dead := &grpcd.DiscoverRequest{
			Step: &grpcd.DiscoverRequest_DeadAddress{DeadAddress: address},
		}

		if err = stream.Send(dead); err != nil {
			log.ErrorContext(ctx, "Failed to report the candidate dead", slog.Any("error", err))

			return "", false
		}
	}
}

// await blocks until the transport to address drops or ctx ends. A drop of
// some other address is a transport grpc-go closed because the address it
// was for had been replaced, which is not a loss.
func (l *loop) await(ctx context.Context, address string) {
	for {
		select {
		case <-ctx.Done():
			return
		case dropped := <-l.upstream.sensor.dropped:
			if dropped != address {
				continue
			}

			l.upstream.log.InfoContext(ctx, "Upstream connection dropped",
				slog.String("address", address))

			return
		}
	}
}
