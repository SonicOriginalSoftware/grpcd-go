package discover

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// starts builds a loop on service and probe, running under a cancellable
// child of the test context, and answers with everything a test needs to
// drive and observe it.
func starts(
	t *testing.T, service *serviceStub, probe Probe,
) (context.Context, context.CancelFunc, *Upstream, *ccStub, <-chan struct{}) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	u := newUpstream(ctx, service, probe)
	cc := newCCStub()

	return ctx, cancel, u, cc, running(ctx, u, cc)
}

func TestLoop(t *testing.T) {
	t.Run("pushes the first reachable candidate with a watch on it", func(t *testing.T) {
		stream := &discoverStream{candidates: []string{"10.0.0.1:50054"}}
		service := &serviceStub{streams: []*discoverStream{stream}}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		if !stream.wasClosed() {
			t.Error("expected the stream to be closed, which is how grpcd is told the candidate worked")
		}

		if got := stream.reported(); len(got) != 0 {
			t.Errorf("reported %v dead, want none", got)
		}

		// The watch is open before the discovery is closed, so no registration
		// falls between the two.
		if got := service.eventLog(); !slices.Equal(got, []string{"watch:10.0.0.1:50054", "close"}) {
			t.Errorf("events = %v, want the watch before the close", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("reports a dead candidate and takes the next", func(t *testing.T) {
		stream := &discoverStream{candidates: []string{"10.0.0.1:50054", "10.0.0.2:50054"}}
		service := &serviceStub{streams: []*discoverStream{stream}}

		_, cancel, _, cc, done := starts(t, service, probeStub("10.0.0.1:50054"))

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.2:50054")

		if got := stream.reported(); !slices.Equal(got, []string{"10.0.0.1:50054"}) {
			t.Errorf("reported %v dead, want [10.0.0.1:50054]", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("reopens the stream when it ends without an address", func(t *testing.T) {
		service := &serviceStub{streams: []*discoverStream{
			{},
			{candidates: []string{"10.0.0.1:50054"}},
		}}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		if got := service.discovers(); got != 2 {
			t.Errorf("discovered %d times, want 2", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("keeps asking when discovery cannot be opened", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		// Ending the loop on the second attempt shows the first failure was
		// retried rather than fatal.
		service := &serviceStub{err: errors.New("unavailable")}
		service.onCall = func(n int) {
			if n == 2 {
				cancel()
			}
		}

		done := running(ctx, newUpstream(ctx, service, probeStub()), newCCStub())

		await(t, done, "loop did not stop")

		if got := service.discovers(); got < 2 {
			t.Errorf("discovered %d times, want at least 2", got)
		}
	})

	t.Run("keeps asking when the method cannot be sent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		stream := &discoverStream{askErr: errors.New("broken transport")}
		service := &serviceStub{streams: []*discoverStream{stream}}
		service.onCall = func(n int) {
			if n == 2 {
				cancel()
			}
		}

		done := running(ctx, newUpstream(ctx, service, probeStub()), newCCStub())

		await(t, done, "loop did not stop")

		if got := service.discovers(); got < 2 {
			t.Errorf("discovered %d times, want at least 2", got)
		}
	})

	t.Run("gives up the stream when a dead report cannot be sent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		stream := &discoverStream{
			candidates: []string{"10.0.0.1:50054"},
			reportErr:  errors.New("broken transport"),
		}
		service := &serviceStub{streams: []*discoverStream{stream}}
		service.onCall = func(n int) {
			if n == 2 {
				cancel()
			}
		}

		done := running(ctx, newUpstream(ctx, service, probeStub("10.0.0.1:50054")), newCCStub())

		await(t, done, "loop did not stop")

		if got := service.discovers(); got < 2 {
			t.Errorf("discovered %d times, want at least 2", got)
		}
	})

	t.Run("gives up the candidate when the watch cannot open before the process ends", func(t *testing.T) {
		stream := &discoverStream{candidates: []string{"10.0.0.1:50054"}}
		service := &serviceStub{
			streams:    []*discoverStream{stream},
			watchErr:   errors.New("unavailable"),
			watchCalls: make(chan struct{}, 16),
		}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)

		// The watch has been refused at least once before the process ends.
		await(t, service.watchCalls, "watch was never attempted")

		cancel()
		await(t, done, "loop did not stop")

		select {
		case state := <-cc.states:
			t.Errorf("pushed %v, want nothing after the watch failed to open", state.Addresses)
		default:
		}
	})

	t.Run("rediscovers after the transport drops", func(t *testing.T) {
		service := &serviceStub{streams: []*discoverStream{
			{candidates: []string{"10.0.0.1:50054"}},
			{candidates: []string{"10.0.0.2:50054"}},
		}}

		ctx, cancel, u, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		// Connected and gone before the loop looks: what it once had, it no
		// longer has.
		ended(began(ctx, u, "10.0.0.1:50054"), u)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.2:50054")

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("waits through a transport it never pushed ending", func(t *testing.T) {
		service := &serviceStub{streams: []*discoverStream{
			{candidates: []string{"10.0.0.1:50054"}},
			{candidates: []string{"10.0.0.2:50054"}},
		}}

		ctx, cancel, u, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		// A stale transport ending is not the pushed address dropping.
		ended(began(ctx, u, "10.0.0.9:50054"), u)

		// Only the pushed address connecting and dropping rediscovers.
		ended(began(ctx, u, "10.0.0.1:50054"), u)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.2:50054")

		if got := service.discovers(); got != 2 {
			t.Errorf("discovered %d times, want 2", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("moves when told and watches the new address", func(t *testing.T) {
		feed := newWatchStream()
		service := &serviceStub{
			streams: []*discoverStream{{candidates: []string{"10.0.0.1:50054"}}},
			watches: []*watchStream{feed},
		}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		feed.moves <- "10.0.0.2:50054"

		cc.expectAddress(t, "10.0.0.2:50054")

		if got := service.watchedAddresses(); !slices.Equal(got, []string{"10.0.0.1:50054", "10.0.0.2:50054"}) {
			t.Errorf("watched %v, want the old address then the new", got)
		}

		await(t, service.openedWatch(0).ended(), "old watch was not closed after the move")

		if got := service.discovers(); got != 1 {
			t.Errorf("discovered %d times, want 1: a move is not a rediscovery", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("stays when told an address it cannot reach", func(t *testing.T) {
		feed := newWatchStream()
		service := &serviceStub{
			streams: []*discoverStream{{candidates: []string{"10.0.0.1:50054"}}},
			watches: []*watchStream{feed},
		}

		_, cancel, _, cc, done := starts(t, service, probeStub("10.0.0.2:50054"))

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		feed.moves <- "10.0.0.2:50054"
		feed.moves <- "10.0.0.3:50054"

		cc.expectAddress(t, "10.0.0.3:50054")

		if got := service.watchedAddresses(); !slices.Equal(got, []string{"10.0.0.1:50054", "10.0.0.3:50054"}) {
			t.Errorf("watched %v, want no watch on the unreachable address", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("ignores the old transport ending after a move", func(t *testing.T) {
		feed := newWatchStream()
		service := &serviceStub{
			streams: []*discoverStream{
				{candidates: []string{"10.0.0.1:50054"}},
				{candidates: []string{"10.0.0.3:50054"}},
			},
			watches: []*watchStream{feed},
		}

		ctx, cancel, u, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		old := began(ctx, u, "10.0.0.1:50054")

		feed.moves <- "10.0.0.2:50054"

		cc.expectAddress(t, "10.0.0.2:50054")

		// grpc-go connects the new address and lets the old transport drain
		// on its own time. Its ending is not the new address dropping.
		current := began(ctx, u, "10.0.0.2:50054")
		ended(old, u)

		// The new address dropping is.
		ended(current, u)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.3:50054")

		if got := service.discovers(); got != 2 {
			t.Errorf("discovered %d times, want 2", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("reopens the watch when it ends", func(t *testing.T) {
		service := &serviceStub{
			streams:    []*discoverStream{{candidates: []string{"10.0.0.1:50054"}}},
			watches:    []*watchStream{{moves: make(chan string), recvErr: errors.New("broken transport")}, newWatchStream()},
			watchCalls: make(chan struct{}, 16),
		}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		await(t, service.watchCalls, "first watch never opened")
		await(t, service.watchCalls, "watch was not reopened after ending")

		if got := service.watchedAddresses(); !slices.Equal(got, []string{"10.0.0.1:50054", "10.0.0.1:50054"}) {
			t.Errorf("watched %v, want the same address twice", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("gives up a move when the new watch cannot open before the process ends", func(t *testing.T) {
		feed := newWatchStream()
		service := &serviceStub{
			streams:    []*discoverStream{{candidates: []string{"10.0.0.1:50054"}}},
			watches:    []*watchStream{feed},
			watchCalls: make(chan struct{}, 16),
		}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		await(t, service.watchCalls, "first watch never opened")

		service.setWatchErr(errors.New("unavailable"))

		feed.moves <- "10.0.0.2:50054"

		// The new watch has been refused at least once.
		await(t, service.watchCalls, "new watch was never attempted")

		// Read by the old watcher while the loop is stuck waiting for the new
		// watch, so the watcher is holding a move nobody takes when the
		// process ends.
		feed.moves <- "10.0.0.3:50054"

		cancel()
		await(t, done, "loop did not stop")

		select {
		case state := <-cc.states:
			t.Errorf("pushed %v, want nothing after the watch failed to open", state.Addresses)
		default:
		}
	})

	t.Run("returns from holding when its watcher stops", func(t *testing.T) {
		service := &serviceStub{}
		u := newUpstream(t.Context(), service, probeStub())
		l := &loop{upstream: u, cc: newCCStub(), cancel: func() {}}

		stopped, stop := context.WithCancel(t.Context())
		w := l.watch(stopped, "10.0.0.1:50054")
		stop()

		if got := l.hold(t.Context(), "10.0.0.1:50054", w); got != w {
			t.Errorf("hold answered %v, want the watcher it was given", got)
		}
	})

	t.Run("stops when the process ends while waiting", func(t *testing.T) {
		service := &serviceStub{streams: []*discoverStream{
			{candidates: []string{"10.0.0.1:50054"}},
		}}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("stops when the process ends while discovering", func(t *testing.T) {
		service := &serviceStub{blocks: true, released: make(chan struct{})}

		_, cancel, _, cc, done := starts(t, service, probeStub())

		cc.expectNone(t)

		cancel()
		await(t, service.released, "discovery was not released")
		await(t, done, "loop did not stop")
	})
}
