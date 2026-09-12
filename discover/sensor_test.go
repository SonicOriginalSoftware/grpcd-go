package discover

import (
	"context"
	"testing"

	"google.golang.org/grpc/stats"

	"git.sonicoriginal.software/grpc-testing/mocks/addr"
)

// open tells s a transport to address has opened, the way grpc-go does: tag
// the context, then hand the event over on it.
func open(ctx context.Context, s *sensor, address string) context.Context {
	ctx = s.TagConn(ctx, &stats.ConnTagInfo{RemoteAddr: addr.New(address)})
	s.HandleConn(ctx, &stats.ConnBegin{Client: true})

	return ctx
}

// signalled reports whether s has raised its flag, taking it.
func signalled(s *sensor) bool {
	select {
	case <-s.changed:
		return true
	default:
		return false
	}
}

func TestSensor(t *testing.T) {
	t.Run("knows nothing before a transport opens", func(t *testing.T) {
		s := newSensor()

		if got := s.State(); got != (view{}) {
			t.Errorf("state = %+v, want empty", got)
		}

		if signalled(s) {
			t.Error("expected no signal before anything happened")
		}
	})

	t.Run("records the peer once a transport opens", func(t *testing.T) {
		s := newSensor()

		open(t.Context(), s, "10.0.0.1:50054")

		want := view{current: "10.0.0.1:50054", lastBegan: "10.0.0.1:50054"}
		if got := s.State(); got != want {
			t.Errorf("state = %+v, want %+v", got, want)
		}

		if got := s.Address(); got != "10.0.0.1:50054" {
			t.Errorf("address = %q, want 10.0.0.1:50054", got)
		}

		if !signalled(s) {
			t.Error("expected a signal")
		}
	})

	t.Run("forgets the current transport when it closes and remembers it began", func(t *testing.T) {
		s := newSensor()

		ctx := open(t.Context(), s, "10.0.0.1:50054")
		s.HandleConn(ctx, &stats.ConnEnd{Client: true})

		want := view{lastBegan: "10.0.0.1:50054"}
		if got := s.State(); got != want {
			t.Errorf("state = %+v, want %+v", got, want)
		}

		if got := s.Address(); got != "" {
			t.Errorf("address = %q, want empty", got)
		}
	})

	t.Run("keeps the current transport when another closes", func(t *testing.T) {
		s := newSensor()

		old := open(t.Context(), s, "10.0.0.1:50054")
		open(t.Context(), s, "10.0.0.2:50054")
		s.HandleConn(old, &stats.ConnEnd{Client: true})

		want := view{current: "10.0.0.2:50054", lastBegan: "10.0.0.2:50054"}
		if got := s.State(); got != want {
			t.Errorf("state = %+v, want %+v", got, want)
		}
	})

	t.Run("raises the flag once for several changes", func(t *testing.T) {
		s := newSensor()

		ctx := open(t.Context(), s, "10.0.0.1:50054")
		s.HandleConn(ctx, &stats.ConnEnd{Client: true})
		s.HandleConn(ctx, &stats.ConnEnd{Client: true})

		if !signalled(s) {
			t.Fatal("expected a signal")
		}

		if signalled(s) {
			t.Error("expected the changes to raise the flag once")
		}
	})

	t.Run("signals a close it did not track, changing nothing", func(t *testing.T) {
		s := newSensor()

		// Never began as far as the sensor knows; the loop decides what a
		// close means, so it is still told to look.
		s.HandleConn(t.Context(), &stats.ConnEnd{Client: true})

		if got := s.State(); got != (view{}) {
			t.Errorf("state = %+v, want empty", got)
		}

		if !signalled(s) {
			t.Error("expected a signal")
		}
	})

	t.Run("ignores RPC events", func(t *testing.T) {
		s := newSensor()

		ctx := s.TagRPC(t.Context(), &stats.RPCTagInfo{})
		s.HandleRPC(ctx, &stats.Begin{})

		if signalled(s) {
			t.Error("expected no signal for an RPC")
		}
	})
}
