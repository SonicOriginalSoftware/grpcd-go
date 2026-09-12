package discover

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestLoop(t *testing.T) {
	t.Run("pushes the first reachable candidate", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		stream := &discoverStream{candidates: []string{"10.0.0.1:50054"}}
		service := &serviceStub{streams: []*discoverStream{stream}}
		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		if !stream.wasClosed() {
			t.Error("expected the stream to be closed, which is how grpcd is told the candidate worked")
		}

		if got := stream.reported(); len(got) != 0 {
			t.Errorf("reported %v dead, want none", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("reports a dead candidate and takes the next", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		stream := &discoverStream{candidates: []string{"10.0.0.1:50054", "10.0.0.2:50054"}}
		service := &serviceStub{streams: []*discoverStream{stream}}
		u := newUpstream(ctx, service, probeStub("10.0.0.1:50054"))
		cc := newCCStub()

		done := running(ctx, u, cc)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.2:50054")

		if got := stream.reported(); !slices.Equal(got, []string{"10.0.0.1:50054"}) {
			t.Errorf("reported %v dead, want [10.0.0.1:50054]", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("reopens the stream when it ends without an address", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{streams: []*discoverStream{
			{},
			{candidates: []string{"10.0.0.1:50054"}},
		}}
		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

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

		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

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

		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

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

		u := newUpstream(ctx, service, probeStub("10.0.0.1:50054"))
		cc := newCCStub()

		done := running(ctx, u, cc)

		await(t, done, "loop did not stop")

		if got := service.discovers(); got < 2 {
			t.Errorf("discovered %d times, want at least 2", got)
		}
	})

	t.Run("rediscovers after the connection drops", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{streams: []*discoverStream{
			{candidates: []string{"10.0.0.1:50054"}},
			{candidates: []string{"10.0.0.2:50054"}},
		}}
		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		drop(ctx, u, "10.0.0.1:50054")

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.2:50054")

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("ignores a drop of an address it is not on", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{streams: []*discoverStream{
			{candidates: []string{"10.0.0.1:50054"}},
			{candidates: []string{"10.0.0.2:50054"}},
		}}
		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		drop(ctx, u, "10.0.0.9:50054")

		// Queued behind the drop above, so it is delivered only once the loop
		// has read and ignored that one.
		u.sensor.dropped <- "10.0.0.1:50054"

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.2:50054")

		if got := service.discovers(); got != 2 {
			t.Errorf("discovered %d times, want 2", got)
		}

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("stops when the process ends while waiting", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{streams: []*discoverStream{
			{candidates: []string{"10.0.0.1:50054"}},
		}}
		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

		cc.expectNone(t)
		cc.expectAddress(t, "10.0.0.1:50054")

		cancel()
		await(t, done, "loop did not stop")
	})

	t.Run("stops when the process ends while discovering", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := &serviceStub{blocks: true, released: make(chan struct{})}
		u := newUpstream(ctx, service, probeStub())
		cc := newCCStub()

		done := running(ctx, u, cc)

		cc.expectNone(t)

		cancel()
		await(t, service.released, "discovery was not released")
		await(t, done, "loop did not stop")
	})
}
