package discover

import (
	"testing"

	"google.golang.org/grpc/resolver"

	foundationclient "git.sonicoriginal.software/grpc-foundation/client"
)

func TestNew(t *testing.T) {
	t.Run("substitutes a logger and a probe when given none", func(t *testing.T) {
		d := New(t.Context(), nil, &serviceStub{}, nil)

		if d.log == nil {
			t.Error("expected a logger")
		}

		if d.probe == nil {
			t.Error("expected a probe")
		}
	})
}

func TestUpstream(t *testing.T) {
	t.Run("targets the service under the grpcd scheme", func(t *testing.T) {
		u := newUpstream(t.Context(), &serviceStub{}, probeStub())

		if got := u.Target(); got != "grpcd:///pkg.Service" {
			t.Errorf("target = %q, want grpcd:///pkg.Service", got)
		}
	})

	t.Run("builds a client with its dial options", func(t *testing.T) {
		u := newUpstream(t.Context(), &serviceStub{}, probeStub())

		// Building a client performs no I/O, so this proves the target parses
		// and the options are accepted without anything being dialled.
		conn, err := foundationclient.New(u.Target(), nil, nil, u.DialOptions()...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := conn.Close(); err != nil {
			t.Errorf("unexpected error closing: %v", err)
		}
	})

	t.Run("reports no address before anything is connected", func(t *testing.T) {
		u := newUpstream(t.Context(), &serviceStub{}, probeStub())

		if got := u.Address(); got != "" {
			t.Errorf("address = %q, want empty", got)
		}
	})

	t.Run("builds a resolver whose close stops the loop", func(t *testing.T) {
		service := &serviceStub{blocks: true, released: make(chan struct{})}
		u := newUpstream(t.Context(), service, probeStub())
		cc := newCCStub()

		r, err := u.Build(resolver.Target{}, cc, resolver.BuildOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		cc.expectNone(t)

		// A hint, and ignored: the loop is still blocked in discovery after it.
		r.ResolveNow(resolver.ResolveNowOptions{})

		r.Close()

		await(t, service.released, "closing the resolver did not stop the loop")
	})
}
