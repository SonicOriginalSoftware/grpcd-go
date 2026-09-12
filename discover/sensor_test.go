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

func TestSensor(t *testing.T) {
	t.Run("reports no address before a transport opens", func(t *testing.T) {
		s := newSensor()

		if got := s.Address(); got != "" {
			t.Errorf("address = %q, want empty", got)
		}
	})

	t.Run("reports the peer once a transport opens", func(t *testing.T) {
		s := newSensor()

		open(t.Context(), s, "10.0.0.1:50054")

		if got := s.Address(); got != "10.0.0.1:50054" {
			t.Errorf("address = %q, want 10.0.0.1:50054", got)
		}
	})

	t.Run("clears the address and reports the drop when a transport closes", func(t *testing.T) {
		s := newSensor()

		ctx := open(t.Context(), s, "10.0.0.1:50054")
		s.HandleConn(ctx, &stats.ConnEnd{Client: true})

		if got := s.Address(); got != "" {
			t.Errorf("address = %q, want empty", got)
		}

		select {
		case dropped := <-s.dropped:
			if dropped != "10.0.0.1:50054" {
				t.Errorf("dropped = %q, want 10.0.0.1:50054", dropped)
			}
		default:
			t.Fatal("expected the drop to be reported")
		}
	})

	t.Run("does not block when a drop is already waiting", func(t *testing.T) {
		s := newSensor()

		ctx := open(t.Context(), s, "10.0.0.1:50054")
		s.HandleConn(ctx, &stats.ConnEnd{Client: true})
		s.HandleConn(ctx, &stats.ConnEnd{Client: true})

		if got := len(s.dropped); got != 1 {
			t.Errorf("queued drops = %d, want 1", got)
		}
	})

	t.Run("ignores what it is not asked about", func(t *testing.T) {
		s := newSensor()

		ctx := s.TagRPC(t.Context(), &stats.RPCTagInfo{})
		s.HandleRPC(ctx, &stats.Begin{})

		if got := s.Address(); got != "" {
			t.Errorf("address = %q, want empty", got)
		}
	})
}
